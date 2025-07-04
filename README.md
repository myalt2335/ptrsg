# PTRSG
One of the tools of all time.

Currently, it only supports **Windows** because it hardcodes `.exe` when generating temporary files. Cross-platform support may come later.

## How 2
You run ptrsg.exe in your console. Like this:
```
> .\ptrsg.exe
Seed generated (512-bit): 3141033219853367287786195825457119086101439969914952726864609394847370129275364207110803252057378215758712475337718100589365841083038448680529399309660126
```
## IMPORTANT

If you for some reason want to use this tool, you need a **LOT** of code language runtimes.

These are direct links to installers for each language runtime/compiler:

- [Python 3.13.5 (64-bit)](https://www.python.org/ftp/python/3.13.5/python-3.13.5-amd64.exe)
- [C++ Redistributable (MSVC)](https://aka.ms/vs/17/release/vc_redist.x64.exe)
- [Node.js v22.17.0 (64-bit MSI)](https://nodejs.org/dist/v22.17.0/node-v22.17.0-x64.msi)
- **Lua** — No direct link; use a package manager like [Scoop](https://scoop.sh) (`scoop install lua`) or [LuaBinaries](https://sourceforge.net/projects/luabinaries/)
- [Rust (via rustup-init.exe)](https://static.rust-lang.org/rustup/dist/x86_64-pc-windows-msvc/rustup-init.exe)
- [Go 1.24.4 (64-bit MSI)](https://go.dev/dl/go1.24.4.windows-amd64.msi)

## Flags

PTRSG supports the following flags:

- `--verbose [none|lite|heavy]`  
  Controls logging output.  
  `none` (default), `lite` shows some useful info, `heavy` logs everything.

- `--queue`  
  Run each language one at a time instead of in parallel. Might reduce CPU strain.

- `--chaos [low|high]`  
  Controls how many languages are used.  
  `low` uses a few core ones, `high` (default) includes all.

- `-S <1-512>`  
  Specifies how long the output seed should be (in bits).  
  Example: `-S 128` for a 128-bit seed.
