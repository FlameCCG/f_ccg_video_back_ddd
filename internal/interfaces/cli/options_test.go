package cli

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // Parsing temporarily replaces flag.CommandLine and os.Args.
func TestParseOptionsAll(t *testing.T) {
	originalFlags, originalArgs := flag.CommandLine, os.Args

	t.Cleanup(func() {
		flag.CommandLine, os.Args = originalFlags, originalArgs
	})

	for _, tt := range []struct {
		name string
		args []string
		want Task
	}{
		{"default serves", nil, TaskServe},
		{"all", []string{"-all"}, TaskInitAll},
		{"double dash all", []string{"--all"}, TaskInitAll},
		{"all disabled", []string{"-all=false"}, TaskServe},
		{"all with individual flag", []string{"-all", "-db"}, TaskInitAll},
	} {
		//nolint:paralleltest // Each case replaces the same process flag set and argument vector.
		t.Run(tt.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet("server", flag.ContinueOnError)

			os.Args = append([]string{"server", "-f", "custom.yaml"}, tt.args...)

			opts := ParseOptions()
			require.Equal(t, "custom.yaml", opts.File, "config path = %q, want custom.yaml", opts.File)

			{
				got := ResolveTask(opts)
				require.Equal(t, tt.want, got, "resolved task = %v, want %v", got, tt.want)
			}
		})
	}
}

//nolint:paralleltest // Parsing temporarily replaces flag.CommandLine and os.Args.
func TestParseOptionsDefaultConfig(t *testing.T) {
	originalFlags, originalArgs := flag.CommandLine, os.Args

	t.Cleanup(func() {
		flag.CommandLine, os.Args = originalFlags, originalArgs
	})

	for _, tt := range []struct {
		name string
		args []string
		want Task
	}{
		{"serve uses development config", nil, TaskServe},
		{"all uses development config", []string{"-all", "test-admin", "test-password"}, TaskInitAll},
	} {
		//nolint:paralleltest // Each case replaces the same process flag set and argument vector.
		t.Run(tt.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet("server", flag.ContinueOnError)

			os.Args = append([]string{"server"}, tt.args...)

			opts := ParseOptions()
			require.Equal(t, "config_dev.yaml", opts.File, "default config path = %q, want config_dev.yaml", opts.File)
			require.Equal(t, "config_dev.yaml", ProvideConfigPath(opts), "default config path = %q, want config_dev.yaml", opts.File)

			{
				got := ResolveTask(opts)
				require.Equal(t, tt.want, got, "resolved task = %v, want %v", got, tt.want)
			}

			require.False(t, opts.All && (len(opts.Args) != 2 || opts.Args[0] != "test-admin" || opts.Args[1] != "test-password"), "all must preserve super user arguments")
		})
	}
}
