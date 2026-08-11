/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - SSE Chat Streaming Helper
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

/**
 * Normalises a tool call's arguments as they arrive on a
 * `tool_use_start` event into the same partial-JSON string that
 * `tool_use_delta` chunks accumulate, so both delivery styles converge
 * on one representation before being parsed.
 *
 * A missing or null `input` yields the empty string, because the wire
 * format sends `input: null` for the providers that stream arguments as
 * deltas instead. Arguments already encoded as a JSON string are passed
 * through untouched rather than re-encoded, which would otherwise parse
 * back to a quoted string instead of an object.
 *
 * @param {*} input - The start event's `tool_use.input`, if any.
 * @returns {string} Partial JSON to seed the accumulator with.
 */
function encodeStartArgs(input) {
    if (input == null) return '';
    if (typeof input === 'string') return input;
    return JSON.stringify(input);
}

/**
 * Posts to the library proxy's streaming chat endpoint
 * (/api/llm/v1/chat/stream), parses Server-Sent Events, and assembles
 * the final response into the same shape returned by the non-streaming
 * /v1/chat endpoint: { content, stop_reason, usage }.
 *
 * SSE frames are separated by blank lines (\n\n). Each frame may
 * contain an `event:` line and one or more `data:` lines (which are
 * joined with \n). The default event type when none is specified is
 * "message". Special event types handled here:
 *
 *   - "done"  -> finalises the assembled response (carries stop_reason
 *                and usage).
 *   - "error" -> aborts the stream and rejects the returned promise.
 *
 * Chunk types within "message" events:
 *
 *   - "text"            -> appended to the current text block; also
 *                          surfaced via the onTextChunk callback so the
 *                          UI can update incrementally.
 *   - "tool_use_start"  -> begins a new tool_use block (id, name, and
 *                          an optional provider-specific signature that
 *                          must be carried through unchanged; Gemini's
 *                          thinking models require it back on this same
 *                          call the next time it appears in history). May
 *                          also carry the complete arguments directly, for
 *                          providers (Gemini, Ollama) that never stream
 *                          them incrementally.
 *   - "tool_use_delta"  -> accumulates partial JSON input string for
 *                          the current tool_use; parsed at done. Providers
 *                          that deliver complete arguments on the start
 *                          event send no delta chunks at all.
 *
 * @param {object} body - Request body matching the /v1/chat schema
 *     (messages, tools, provider, model, etc.).
 * @param {object} [options] - Optional knobs.
 * @param {AbortSignal} [options.signal] - Abort signal forwarded to
 *     fetch.
 * @param {string} [options.sessionToken] - Bearer token used for the
 *     Authorization header.
 * @param {Function} [options.onTextChunk] - Called with each text
 *     fragment (string) as it arrives, suitable for incremental UI
 *     updates.
 * @returns {Promise<object>} Resolves with the assembled
 *     { content, stop_reason, usage } response.
 */
export async function sseChat(body, options = {}) {
    const { signal, sessionToken, onTextChunk } = options;

    const headers = {
        'Content-Type': 'application/json',
        'Accept': 'text/event-stream',
    };
    if (sessionToken) {
        headers['Authorization'] = `Bearer ${sessionToken}`;
    }

    const response = await fetch('/api/llm/v1/chat/stream', {
        method: 'POST',
        headers,
        credentials: 'include',
        signal,
        body: JSON.stringify(body),
    });

    if (!response.ok) {
        const text = await response.text();
        const err = new Error(`HTTP ${response.status}: ${text}`);
        err.status = response.status;
        err.body = text;
        throw err;
    }

    if (!response.body || typeof response.body.getReader !== 'function') {
        throw new Error('Streaming response body is not readable');
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    // Assembly state mirrors the non-streaming endpoint's response.
    const assembled = {
        content: [],
        stop_reason: 'end_turn',
        usage: null,
    };
    let pendingTextBlock = null;
    // Ordered list of tool_use ids so we preserve emission order at done.
    const toolOrder = [];
    // Map of tool_use id -> { name, partial, signature } accumulator.
    const pendingTools = new Map();
    let currentToolId = null;
    let streamError = null;

    const flushPendingText = () => {
        if (pendingTextBlock) {
            assembled.content.push(pendingTextBlock);
            pendingTextBlock = null;
        }
    };

    const finalise = () => {
        flushPendingText();
        for (const id of toolOrder) {
            const info = pendingTools.get(id);
            if (!info) continue;
            let input = {};
            const partial = info.partial || '';
            if (partial.trim().length > 0) {
                try {
                    input = JSON.parse(partial);
                } catch (_err) {
                    // Leave input as the raw partial string so the
                    // caller can still inspect what was attempted.
                    input = { _raw: partial };
                }
            }
            const toolUse = { id, name: info.name, input };
            // Gemini's thinking models attach an opaque signature to a
            // function call and require it echoed back unchanged on
            // that same call the next time it appears in conversation
            // history, or the next turn is rejected outright; carry it
            // through rather than reconstructing the block without it.
            if (info.signature) {
                toolUse.signature = info.signature;
            }
            assembled.content.push({
                type: 'tool_use',
                tool_use: toolUse,
            });
        }
    };

    const handleEvent = (eventType, dataLines) => {
        if (dataLines.length === 0) return;
        const dataStr = dataLines.join('\n');
        let parsed;
        try {
            parsed = JSON.parse(dataStr);
        } catch (_err) {
            // Ignore malformed payloads; the server may emit comments
            // or heartbeats we don't recognise.
            return;
        }

        if (eventType === 'done') {
            if (parsed.stop_reason) {
                assembled.stop_reason = parsed.stop_reason;
            }
            if (parsed.usage) {
                assembled.usage = parsed.usage;
            }
            // If the done payload also carries assembled content
            // blocks, prefer the server's view.
            if (Array.isArray(parsed.content) && parsed.content.length > 0) {
                assembled.content = parsed.content;
                // Don't run finalise() in this case; the server already
                // provided the assembled shape.
                pendingTextBlock = null;
                pendingTools.clear();
                toolOrder.length = 0;
            } else {
                finalise();
            }
            return;
        }

        if (eventType === 'error') {
            const msg = parsed.error || parsed.message || 'stream error';
            streamError = new Error(msg);
            return;
        }

        // Default "message" events carry chunk payloads.
        switch (parsed.type) {
            case 'text': {
                if (!pendingTextBlock) {
                    pendingTextBlock = { type: 'text', text: '' };
                }
                const chunk = parsed.text || '';
                pendingTextBlock.text += chunk;
                if (chunk && typeof onTextChunk === 'function') {
                    try {
                        onTextChunk(chunk);
                    } catch (_err) {
                        // Don't let UI callbacks abort the stream.
                    }
                }
                break;
            }
            case 'tool_use_start': {
                // Flush any pending text so the assembled content
                // preserves the relative ordering of blocks.
                flushPendingText();
                const tu = parsed.tool_use || {};
                const id = tu.id || `tu_${pendingTools.size}`;
                currentToolId = id;
                if (!pendingTools.has(id)) {
                    toolOrder.push(id);
                }
                pendingTools.set(id, {
                    name: tu.name || '',
                    // Some providers (Gemini, Ollama) deliver the
                    // complete arguments on this event and send no delta
                    // chunks at all; seed the buffer from them when
                    // present rather than assuming a delta always
                    // follows. The wire format sends `input: null` (no
                    // omitempty on the Go side) for providers still to
                    // come via deltas, so null must be treated the same
                    // as absent rather than as a literal complete value.
                    // A provider sending pre-encoded arguments as a JSON
                    // string is taken as-is, since re-encoding it would
                    // yield a quoted string rather than the object the
                    // caller expects.
                    partial: encodeStartArgs(tu.input),
                    signature: tu.signature || '',
                });
                break;
            }
            case 'tool_use_delta': {
                const id = parsed.id || currentToolId;
                if (id && pendingTools.has(id)) {
                    const info = pendingTools.get(id);
                    info.partial += parsed.partial || '';
                }
                break;
            }
            default:
                // Other chunk types (image, etc.) ignored for now.
                break;
        }
    };

    const processFrame = (frame) => {
        let eventType = 'message';
        const dataLines = [];
        for (const rawLine of frame.split('\n')) {
            const line = rawLine.replace(/\r$/, '');
            if (line.length === 0) continue;
            if (line.startsWith(':')) continue; // SSE comment
            if (line.startsWith('event:')) {
                eventType = line.slice(6).trim();
            } else if (line.startsWith('data:')) {
                // Per SSE spec, strip a single leading space if present.
                let payload = line.slice(5);
                if (payload.startsWith(' ')) payload = payload.slice(1);
                dataLines.push(payload);
            }
            // Other fields (id:, retry:) are ignored.
        }
        handleEvent(eventType, dataLines);
    };

    try {
        while (true) {
            const { value, done } = await reader.read();
            if (done) break;
            buffer += decoder.decode(value, { stream: true });
            let idx;
            while ((idx = buffer.indexOf('\n\n')) !== -1) {
                const frame = buffer.slice(0, idx);
                buffer = buffer.slice(idx + 2);
                processFrame(frame);
                if (streamError) break;
            }
            if (streamError) break;
        }
        // Flush any trailing data that wasn't terminated by \n\n.
        if (!streamError && buffer.trim().length > 0) {
            processFrame(buffer);
            buffer = '';
        }
    } finally {
        try {
            reader.releaseLock();
        } catch (_err) {
            // ignore
        }
    }

    if (streamError) {
        throw streamError;
    }

    // The streaming done frame carries a StreamChunk, which has no
    // stop_reason field, so the proxy cannot tell us why the model
    // stopped; providers that report finish_reason "tool_calls" (such
    // as OpenAI) therefore arrive here indistinguishable from a plain
    // text turn. A response carrying tool_use blocks is by definition
    // a tool_use turn, so infer that rather than leaving the default
    // "end_turn", which would make callers ignore the tool call. A
    // more specific server-reported reason (max_tokens, for instance)
    // is left alone.
    if (assembled.stop_reason === 'end_turn' &&
        Array.isArray(assembled.content) &&
        assembled.content.some((block) => block && block.type === 'tool_use')) {
        assembled.stop_reason = 'tool_use';
    }

    return assembled;
}

export default sseChat;
