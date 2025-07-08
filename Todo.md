This is a bug. It should be fixed.
This is so stupid that bots are recreated in bots array even in one generation. 

- CLI-based Control panel (pause, speed, generation config) 
- Floating UI for bot info
- UI refactoring -> colors
- Farms, mine, colletors
- Give bots Memory? Communication?

Bugs:
1. Bots can walk into farms and/or food ?
2. Bots can stuck in grabloop around foodfarm.
This is because pointer jumps ending up in to be the same set of positions. 
I decided that this is not somthing I want to fix, but rather let evolution handle it.

----------------------------

New Genome idea

Gene struct:
- opcode
- arg1
- arg2

opcodes:
1 - jump
2 - move
    1 - up
    2 - up-right
    3 - right
    4 - down-right
    5 - down
    6 - down-left
    7 - left
    8 - up-left
    move3 - move right
3 - grab
4 - build
5 - pointer
6 - pointer jump
7 - pointer jump by

Example of genome
int64[move3]

Argument is calculated as:
arg = genome[ptr+1] % 8
