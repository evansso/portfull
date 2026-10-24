# portfull

A small Linux CLI for finding local TCP listeners and stopping the process that owns a port. Portfull is open source under the MIT license.

## Install a release binary

Release downloads are built by GitHub Actions for Linux on `amd64` and `arm64`. Download the archive for your machine from the project's **Releases** page, then extract and install it without Go:

```sh
tar -xzf portfull_<version>_linux_amd64.tar.gz
chmod +x portfull
mkdir -p "$HOME/.local/bin"
mv portfull "$HOME/.local/bin/portfull"
```

Use `linux_arm64` in the archive name for ARM64 devices. Add `~/.local/bin` to your `PATH` if it is not already there, then run `portfull list` from any project.

To run it directly from the extracted folder instead, use `chmod +x portfull` followed by `./portfull list`.

## Build from source

This is mainly for contributors. It requires Go 1.22 or later and Linux.

```sh
git clone https://github.com/evansso/portfull.git
cd portfull
go build -o portfull .
./portfull list
```

## Use

```sh
portfull list
portfull kill 3000
```

`kill` checks whether the port has a listening process, shows its command and PID, and asks before sending it `SIGTERM`. Answer `n` or press Enter to skip. If the port is already free, it reports that and exits without doing anything.

For scripts, `--yes` skips the prompt:

```sh
portfull kill 3000 --yes
```

`list` shows listening TCP ports, their local address, PID, and process command. The process owner must be visible in `/proc`; use `sudo portfull list` if you need to inspect processes owned by another user.

## Scope

Port discovery uses Linux `/proc` directly and has no third-party dependencies. It currently supports Linux only. Release binaries are provided for Linux `amd64` and `arm64`; contributions for macOS and Windows are welcome.
