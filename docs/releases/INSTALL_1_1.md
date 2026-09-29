# Installing Oct 1.1.0

## Supported hosts and prerequisite

The release provides Windows x86-64 and Linux x86-64 archives. The bundled `oct` executable can run and test Oct programs without a repository checkout. Native `oct build` invokes the installed Go toolchain; use the Go version declared by the archive's `runtime/go.mod`.

Download the archive for your host and `checksums.sha256` from the [v1.1.0 release](https://github.com/yuechen-li-dev/oct/releases/tag/v1.1.0). Verify the archive before extracting it.

## Windows PowerShell

```powershell
Get-Content .\checksums.sha256
Get-FileHash .\oct-1.1.0-windows-amd64.zip -Algorithm SHA256
Expand-Archive .\oct-1.1.0-windows-amd64.zip -DestinationPath "$HOME\Apps"
& "$HOME\Apps\oct-1.1.0-windows-amd64\oct.exe" version
```

Compare the SHA-256 reported by `Get-FileHash` with the Windows archive entry in `checksums.sha256`. Add the extracted versioned directory to `PATH` to invoke `oct` from any directory.

## Linux

```sh
sha256sum -c checksums.sha256
mkdir -p "$HOME/.local/opt"
tar -xzf oct-1.1.0-linux-amd64.tar.gz -C "$HOME/.local/opt"
"$HOME/.local/opt/oct-1.1.0-linux-amd64/oct" version
```

Add the extracted versioned directory to `PATH` for regular use.

## First program

Save this as `Main.oct`:

```oct
package Main

fn Main() -> Int {
    Print("hello from Oct")
    return 0
}
```

Then run `oct run Main.oct` or build it with `oct build Main.oct`. The archive includes `LICENSE`, this guide, the compiler `runtime/` module, and Octxiliary `sidecars/`. Wrapper APIs discover bundled sidecars when compiled programs run beside them. For a program elsewhere, set `OCT_WRAPPER_PATH` to the extracted `sidecars` directory.

Replace the versioned directory to upgrade. Remove that directory and its `PATH` entry to uninstall.
