* Always use 4 spaces for indentation of code.

* Always update documentation where necessary:

    * When editing markdown files, always leave a blank line before the first item in any list or sub-list to ensure the lists render properly in tools such as mkdocs.
    * /README.md is a brief summary for users looking at the code:
    * All documentation content must be available in the documentation under /docs.
    * Markdown files in the root directory should use uppercase names (except for the extension). 
    * Markdown files in /docs should have lowercase names.

* Always add tests to exercise new functionality.
    * When running tests to verify changes, always run all tests and check verbose output for failures or errors.
    * Don't tail or otherwise trim test output to both stdout and stderr when running tests, to ensure nothing is missed.
    * Don't modify any tests unless they are expected to fail as a result of the changes being made.

* Remember to ensure that all code changes remain secure:
    * Authentication should be required if enabled.
    * Per-token connection isolation must be maintained.
    * Token expiry must be respected.
    * Always escape inputs from the user to protect against injection attacks, except
        where the purpose of the tool is to allow the user to execute arbitrary
        SQL they have provided.

* Remember to always ensure the read_resource tool is present, and will properly advertise all resources.

