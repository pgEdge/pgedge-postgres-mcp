/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Unusable Model Error Descriptions
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

/**
 * Recognises the errors a provider returns when the selected model cannot
 * hold a text conversation at all, and rewrites them as something that says
 * so.
 *
 * The model list a provider advertises is not confined to chat models.
 * Gemini's `models.list` reports text-to-speech, image, music and agent
 * models alongside the conversational ones, and they are indistinguishable
 * from the metadata, because they all advertise `generateContent` and the API
 * exposes no field describing multi-turn support. Filtering them out belongs
 * with whoever serves the list; until that happens a reader who picks one
 * gets an error about response modalities or an unfamiliar API, which reads
 * like a misconfiguration rather than the wrong kind of model.
 *
 * The patterns below were taken from the live API rather than guessed. Some
 * of these models fail with nothing more specific than "Request contains an
 * invalid argument", which no amount of matching can improve, so this
 * deliberately recognises only the cases that identify themselves and leaves
 * every other error exactly as the provider phrased it.
 */

/**
 * Reads the response modalities a model will accept out of the error it
 * returns when asked for text, e.g. "AUDIO" for a text-to-speech model.
 *
 * @param {string} text - The full error text.
 * @returns {string|null} The modalities, or null when absent.
 */
const acceptedModalities = (text) => {
    const match = text.match(
        /accepts the following combination of response modalities:\s*([\s\S]*)$/i
    );
    if (!match) return null;
    const names = match[1]
        .split('\n')
        .map(line => line.replace(/^[\s*-]+/, '').trim())
        .filter(Boolean);
    return names.length > 0 ? names.join(', ') : null;
};

/**
 * Describes an error that means the chosen model cannot be used for chat.
 *
 * @param {string} errorText - The error as the provider and proxy phrased it.
 * @param {string} [model] - The model that was selected, named in the result
 *     when the error itself does not name it.
 * @returns {string|null} A replacement message, or null when the error is
 *     not one of the recognised cases and should be shown unaltered.
 */
export function describeUnusableModelError(errorText, model = '') {
    if (!errorText || typeof errorText !== 'string') return null;

    const named = model ? `The model ${model}` : 'The selected model';
    const advice = 'Choose a different model from the model selector.';

    // A model that only emits audio or images, such as Gemini's text-to-speech
    // and image models, rejects a request for a text response outright.
    if (/response modalities/i.test(errorText)) {
        const modalities = acceptedModalities(errorText);
        const produces = modalities
            ? ` it produces ${modalities} output rather than text.`
            : ' it does not produce text output.';
        return `${named} cannot be used for chat, because${produces} ${advice}`;
    }

    // Agent and research models are reached through a different API and are
    // not available for chat here at all.
    if (/only supports\s+(?:the\s+)?([A-Za-z][\w\s-]*?)\s*API\b/i.test(errorText)) {
        const [, api] = errorText.match(
            /only supports\s+(?:the\s+)?([A-Za-z][\w\s-]*?)\s*API\b/i
        );
        return `${named} cannot be used for chat, because it is only available `
            + `through the ${api.trim()} API. ${advice}`;
    }

    // Reported against some models historically, and kept because it states
    // the problem plainly when it does appear.
    if (/multi-?turn chat is not enabled/i.test(errorText)) {
        return `${named} cannot be used for chat, because it does not support `
            + `multi-turn conversations. ${advice}`;
    }

    return null;
}

/**
 * Returns the text to show for a failed chat request: the description above
 * when the error is a recognised unusable-model case, and otherwise the
 * provider's own message untouched, so that nothing this does not understand
 * is reworded or hidden.
 *
 * @param {Error|object} err - The error thrown by the chat call.
 * @param {string} [model] - The model that was selected.
 * @param {string} [fallback] - Text for an error that carries no message at
 *     all, which differs between sending a message and running a prompt.
 * @returns {string} Text to display to the user.
 */
export function formatChatError(err, model = '', fallback = 'Failed to send message') {
    const raw = (err && err.message) || '';
    return describeUnusableModelError(raw, model) || raw || fallback;
}
