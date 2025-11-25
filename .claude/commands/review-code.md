---
allowed-tools: Bash(make:*), Bash(gh:*)
description: Review the entire codebase for issues and improvements
---

# Review Code

## Review all code

    * Look for, and fix any code duplication
    * Ensure the code is well structured and modularised
    * Ensure the code is easily readable by a human
    * Ensure that secure and defensive coding practices are employed
    * Ensure that all MCP server endpoints (except authenticate_user and 
        /health) require authentication
    * Ensure that all database operations are executed in READ ONLY mode, and 
        that the LLM cannot switch into READ/WRITE mode
    * Ensure code follows best practives for the language used
    * Ensure all code (but not configuration files) include the standard 
        copyright/license header
    * Remove any unused code

## Review all documentation

    * Ensure we always use the name "pgEdge Natural Language Agent" or 
        "Natural Language Agent" (where a logo image provides the "pgEdge" 
        text) for the project
    * Ensure links to files outside of /docs link to the copy on GitHub
    * Ensure all tool, resource, and prompts are properly documented, and 
        counts are correct
    * Ensure all sample output matches what what would actually be output
    * Ensure all command line options are documented
    * Ensure all we have configuration examples for all configuration files, 
        and that they contain well commented examples of all options
    * Ensure the sample applications in the docs and /examples are up to date, 
        will work as expected, and show how to use tools, prompts, and 
        resources
    * Ensure the doc structure separates content for End Users (users of the 
        MCP server and clients), Application Developers (builders of 
         applications that integrate the MCP server), and Project Hackers 
         (developers that work on this project)
    * Ensure all READMEs give just a brief overview of key points they need to 
        convey, aimed primarily (but not exclusively) at Project Hackers
    * Ensure changelog.md has been updated to include notable changes made 
        since the last release

## Review all tests

    * Ensure we have maximum unit and integration test coverage
    * Ensure ALL test suites are run by "make test"
    * Ensure all linters have been run
    * Ensure gofmt has been run

