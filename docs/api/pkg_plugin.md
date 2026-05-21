# `github.com/ricardo/anthrogo/pkg/plugin`

```go
package plugin // import "github.com/ricardo/anthrogo/pkg/plugin"


TYPES

type CommandSpec struct {
	Name        string   `yaml:"name"`
	Aliases     []string `yaml:"aliases,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Type        string   `yaml:"type"`
	Body        string   `yaml:"body"`
}
    CommandSpec describes a single plugin-contributed slash command.

type DynamicCommand struct {
	// Has unexported fields.
}
    DynamicCommand implements command.Command from a manifest CommandSpec.

func (d DynamicCommand) Aliases() []string

func (d DynamicCommand) Description() string

func (d DynamicCommand) Name() string

func (d DynamicCommand) Run(_ context.Context, args string, _ command.Host) (command.Result, error)

func (d DynamicCommand) Type() command.Type

type Manifest struct {
	Name        string                         `yaml:"name"`
	Version     string                         `yaml:"version,omitempty"`
	Description string                         `yaml:"description,omitempty"`
	Author      string                         `yaml:"author,omitempty"`
	Commands    []CommandSpec                  `yaml:"commands,omitempty"`
	Skills      []SkillRef                     `yaml:"skills,omitempty"`
	Hooks       hooks.Config                   `yaml:"hooks,omitempty"`
	MCPServers  map[string]mcp.MCPServerConfig `yaml:"mcpServers,omitempty"`
}
    Manifest is the parsed plugin.yaml.

type Plugin struct {
	Name     string
	Manifest Manifest
	BasePath string
	Source   string // "home" | "cwd"

	// Resolved contributions — populated by the loader after path resolution.
	Commands   []command.Command
	Skills     []skill.Skill
	Hooks      hooks.Config
	MCPServers map[string]mcp.MCPServerConfig
}
    Plugin is a resolved, loaded plugin.

func LoadAll(homeRoot, cwdRoot string) ([]Plugin, []string, error)
    LoadAll scans homeRoot and cwdRoot for plugin directories. cwd-level
    plugins override home-level plugins with the same name. Returns the merged
    plugin list, per-plugin warnings, and only returns a top-level error for
    unrecoverable IO.

type Registry struct {
	// Has unexported fields.
}
    Registry holds all loaded plugins, thread-safe.

func NewRegistry(list []Plugin) *Registry
    NewRegistry constructs a Registry from a pre-loaded plugin list.

func (r *Registry) Get(name string) (Plugin, bool)
    Get returns a plugin by name.

func (r *Registry) Install(src, destRoot string) (Plugin, []string, error)
    Install installs a plugin from src into destRoot. src may be:
      - a local directory path (default, M4.4 behaviour)
      - an https:// or http:// URL pointing to a .tar.gz/.tgz/.zip archive
      - a git+https:// or git+ssh:// spec (optionally with @branch suffix)

func (r *Registry) List() []Plugin
    List returns all plugins sorted by name.

func (r *Registry) Reload(homeRoot, cwdRoot string) ([]string, error)
    Reload re-scans both roots and replaces the registry contents atomically.

func (r *Registry) Remove(name, homeRoot string) error
    Remove deletes the home-rooted plugin <name> and unregisters it.

type SkillRef struct {
	Dir string `yaml:"dir"`
}
    SkillRef points to a skill directory relative to the plugin root.

```
