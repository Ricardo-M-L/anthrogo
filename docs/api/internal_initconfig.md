# `github.com/ricardo/anthrogo/internal/initconfig`

```go
package initconfig // import "github.com/ricardo/anthrogo/internal/initconfig"


FUNCTIONS

func Run(in io.Reader, out io.Writer, settingsPath string, force bool) error
    Run launches the interactive wizard. in/out are stdin/stdout (overridable
    for tests). settingsPath is where to write. If file exists and !force,
    return an error describing the conflict.

```
