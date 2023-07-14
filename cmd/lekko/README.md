# lekko CLI

## Design Principles

- Commands can be executed in the interactive mode (within an interactive terminal) as well as
in a non-interactive mode (called from another program or script). Exception to the rule could be the auth commands, as 
they require in some cases cli-browser interaction.
- Arguments which are essential to the proper execution of the command must be passed as positional arguments. 
That applies both to the mandatory and optional arguments. Flags are to be used as modifiers or filters. (This rule has 
been derived from analyzing structure of linux shell commands as well as modern cli programs such as docker and git)

- In the non-interactive mode:
  - interactive prompts for missing information is disabled. If the mandatory arguments are missing or they are 
  determined as malformed, the lekko cli immediately exits with error code 1
  - colors are disabled
  - the lekko cli needs to be called with -i=false or --interactive=false flags to run in the non-interactive mode

- In the interactive mode, if mandatory/optional parameters are not provided on the command line, the user will be prompted to enter them. 
Interactive mode is the default mode of execution. The fallback that prompts the user for the required info is a useful,
user-friendly feature of modern CLIs, and should be supported in the interactive mode (together with the consistent 
use of of colors to improve readability)

- Providing the user with sufficient information is one of the goals while constructing modern cli programs. Generally 
giving the user ample of information of what the command does or regarding the nature of the execution results is encouraged.
- To support the ease and efficiency of parsing, the "quiet" (-q or --quite ) flag can be used to request that only 
essential info, in the simplest, practical format should be produced as output.
- Verbose as well as dry-run options should be available when it is practical and useful


### Changes introduced to lekko cli
- adding non-interactive mode
- adding positional arguments and replacing flags with positional arguments when appropriate
- adding quiet mode for majority of commands
- hiding lekko backend url flag
- normalizing the cli "menus"
- checking for number of passed arguments and generating suitable errors depending on interactive or non-interactive contexts
- including generation of complete helloworld examples in go
- refactoring the code

### Pending changes
- more informative helps
- deciding/implementing more consistent formats for output (in addition to the quite mode/format) - text, lists, tables, json

- ### More info needed
- discussing potential changes to some features
  - auth commands
    - should there be a non-interactive mode implemented there. ANSWER: will look into it later
  - upgrade (key is not needed). DECISION: remove. DONE
  - k8s functionality. DECISION: remove. DONE
  - auto-complete, DECISION: remove for the time being. DONE 
  - adding team delete be implemented(?). DECISION: work on it later
  - auth commands: DECISION: no changes planned for the time being
- more info needed regarding:
  - repo init
  - should --no-colors flags be added
