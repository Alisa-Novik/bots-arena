- CLI-based Control panel (pause, speed, generation config) 
- Floating UI for bot info
- UI refactoring -> colors
- Farms, mine, colletors
- Give bots Memory? Communication?

Ideas for controller tasks
1) find water; 
2) build Mine; 
3) find other colonies; 
4) build wall;
5) 
__
1) Colony should be able to issue high level tasks like "form a connection"
2) Whenever colony (opens Water as a resource?) has connection to a Water
it should be able to build special type of building which is "Windmill" that allows
to recycle extra bots into inventory amount.
3) It should maintain connection to both water and Windmill to maintain benefits.
4) Whenever colony has a Windmill and water it can build Mine and issue "mine" tasks within limit.
It will either decide to assign owners to the task or bots will pick it up on their turn.
Water + Resources -> Farms
________
idea -> idempotent priority based coloring. 
Coloring based on colony membership has more priority than consumptuion based
 
Colony connection is broken
it sets 1) bot.Colony = nil and never connnects it back again. 
Additionnaly there is not distinction between colonies

A lot of stuff is very broken actually. I should go back and fix them after pathfinding in good state.
I need to mark everything I'm finding with todos or leave notes
____

