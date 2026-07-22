# Installing Oct 1.0

## Supported hosts and prerequisite

Oct 1.0 artifacts are provided for Windows x86-64 and Linux x86-64. Native
`oct build` uses the installed Go toolchain as its backend; install the Go
version declared in the bundled `runtime/go.mod` before compiling Oct programs.
No repository checkout is required.

## Verify and install

Windows PowerShell:

```powershell
Get-FileHash .\oct-1.0.0-windows-amd64.zip -Algorithm SHA256
Expand-Archive .\oct-1.0.0-windows-amd64.zip -DestinationPath $HOME\Apps
$env:Path = "$HOME\Apps\oct-1.0.0-windows-amd64;$env:Path"
oct version
```

Linux:

```sh
sha256sum -c checksums.sha256
tar -xzf oct-1.0.0-linux-amd64.tar.gz -C "$HOME/.local/opt"
export PATH="$HOME/.local/opt/oct-1.0.0-linux-amd64:$PATH"
oct version
```

The archive contains `oct`, `LICENSE`, this guide, a compiler `runtime/` module,
and `sidecars/`. Wrapper APIs discover bundled sidecars when a compiled program
runs beside them; otherwise set `OCT_WRAPPER_PATH` to the extracted `sidecars`
directory. Replace the extracted versioned directory to upgrade; delete it and
remove its PATH entry to uninstall.

## First program

```oct
package Main

fn Main() -> Int {
    Print("hello from Oct")
    return 0
}
```

Save it as `Main.oct`, then run:

```sh
oct run Main.oct
oct build Main.oct
./Main.oct.out         # Linux
./Main.oct.exe          # Windows PowerShell: .\Main.oct.exe
oct test . --execution compiled
oct fmt Main.oct
```

`oct fmt` rewrites source in place. Use source control or a copy when reviewing
formatting changes.
