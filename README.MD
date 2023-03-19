```
   __              __  _ __  
  / /_____  ____  / /_(_) /__
 / __/ __ \/ __ \/ __/ / //_/
/ /_/ /_/ / /_/ / /_/ / ,<   
\__/\____/\____/\__/_/_/|_|  
```

## Overview

tootik is a federated federated nanoblogging service with a Gemini frontend.

It's a single executable that handles both the federation (using ActivityPub) and the frontend (using Gemini) aspects, while [sqlite](https://sqlite.org/) takes care of persistency. It should be lightweight and efficient enough to host a small community even on a cheap server, and hopefully, easy to hack on.

tootik allows Gemini users to participate in the fediverse using their Gemini client of choice and makes the fediverse more accessible.

It implements a small subset of ActivityPub, and probably doesn't really conform to the spec.

**This is very dirty and messy piece of code written as a weekend project.** Use with caution.

## Credits and Legal Information

tootik is free and unencumbered software released under the terms of the [Apache License Version 2.0](https://www.apache.org/licenses/LICENSE-2.0); see LICENSE for the license text.

The ASCII art logo at the top was made using [FIGlet](http://www.figlet.org/).
