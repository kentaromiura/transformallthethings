# transformallthethings
A language-agnostic way to transform all your source files to be processed with other incompatible tools

![Transform all the things meme format](allthethings.jpeg)
* meme originated from http://hyperboleandahalf.blogspot.com . 
Quickly made with kapwing.
Note: It's one of the first internet meme.


Why
===
I want to use [esbuild](https://github.com/evanw/esbuild),
it's a fantastic tool that does what it says it does very well;
it's also super fast, and does many common transform I need.


Unfortunately it doesn't share its internal ast so it's impossible to extend it for my specific needs. until now.


Welcome `transformallthethings`
===
With this tool I want to address the situation where we want to apply specific changes to the code as we can't use some of the tools, this is not specific to `esbuild`, but can work with any tool. 

In fact it can be used as a middleware to make older tools to support newer features, or even translate to different languages/architecture (as long as they don't depend on file names).


I also want to keep things fast because it makes no sense to have a very fast bundler if we slow it down with our transformations, so I heavily rely on caching the transforms (Jest-like) to accomplish this.

This means that the transform can be slow at first but after the first run it will always be as fast as it's possible to read from the disk, until the original file is changed.


It's language-agnostic because I believe writing tools using any language have their advantages.

Why GO?
===

This tool is written in Go for the main reason of how it works: 

I use `FUSE` fs to mount the src folder in a mnt folder,
when looking for making a proof of concept I firstly tried a python example but it wasn't quite working, while the go example worked on the first try, so go it is :)


So, how does it works?
===

If you place a transforms.json file in your working directory 
where you map a _regular expression_ to a _command_, 
and then run `transform SOURCE_FOLDER MOUNT_FOLDER` it will mount your `SOURCE_FOLDER` in the `MOUNT_FOLDER` and will pre-execute _command_ for each file matching the _regular expression_.


_command_ will get the original path as an input and can do what it wants with it, it can parse it as an AST or read as a text, it's stdout will be used as result and stored in a `.cache` folder on your working directory that will only be updated if your source file is newer.

If no transform are needed the file is returned as it is.


Notice how in the example `transform.json` if we replace:

```
 "^/src/.*\\.js$": "./cccchange"
```

with:

```
"^/.*\\.js$": "./cccchange"
```

you will find the node_modules folder inside the .cache


Compile
===
To compile just run make (openbsd | netbsd | linux | freebsd | darwin)


you will generate a `transform` executable for your system, if in windows the linux version might work under WSL (untested yet).
