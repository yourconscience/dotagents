# Install And Build

Use these commands from the repository root:

```bash
make build
sudo make install
```

This installs `x-cli` to `/usr/local/bin` by default.

For local development without installing globally:

```bash
go run .
./x-cli --help
```

If the task is about the skill itself, use the repository's `install-skill.sh` script instead of copying files manually.
