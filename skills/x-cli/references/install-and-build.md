# Install And Build

These commands are for building x-cli from its source repository (github.com/Gladium-AI/x-cli), not from this skills directory.

From the x-cli source repo:

```bash
make build
sudo make install   # installs to /usr/local/bin
```

Or install to ~/bin without sudo:

```bash
make build
cp x-cli ~/bin/
```

Verify installation:

```bash
x-cli --help
```

The skill files in this directory are reference documentation only - the x-cli binary must be installed separately from its source repo.
