# Plugins

See [README — Plugins](https://github.com/Ricardo-M-L/anthrogo#plugins).

Plugins extend anthrogo with new tools and slash commands via a `plugin.yaml` manifest.

## Installing plugins

```bash
# Local path
/plugin install ./path/to/plugin

# Remote tarball
/plugin install https://example.com/plugins/my-plugin.tar.gz

# Git repository
/plugin install git+https://github.com/foo/anthrogo-plugin-git.git
/plugin install git+https://github.com/foo/anthrogo-plugin-git.git@v1.0
```

## plugin.yaml format

```yaml
name: my-plugin
version: 1.0.0
tools:
  - name: MyTool
    description: Does something useful
    command: ./bin/mytool
```

(Full plugin reference migrating from README — M11.4 follow-up.)
