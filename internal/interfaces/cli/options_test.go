package cli

import (
	"flag"
	"os"
	"testing"
)

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
		t.Run(tt.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet("server", flag.ContinueOnError)
			os.Args = append([]string{"server", "-f", "custom.yaml"}, tt.args...)
			opts := ParseOptions()
			if opts.File != "custom.yaml" {
				t.Fatalf("config path = %q, want custom.yaml", opts.File)
			}
			if got := ResolveTask(opts); got != tt.want {
				t.Fatalf("resolved task = %v, want %v", got, tt.want)
			}
		})
	}
}

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
		t.Run(tt.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet("server", flag.ContinueOnError)
			os.Args = append([]string{"server"}, tt.args...)
			opts := ParseOptions()
			if opts.File != "config_dev.yaml" || ProvideConfigPath(opts) != "config_dev.yaml" {
				t.Fatalf("default config path = %q, want config_dev.yaml", opts.File)
			}
			if got := ResolveTask(opts); got != tt.want {
				t.Fatalf("resolved task = %v, want %v", got, tt.want)
			}
			if opts.All && (len(opts.Args) != 2 || opts.Args[0] != "test-admin" || opts.Args[1] != "test-password") {
				t.Fatal("all must preserve super user arguments")
			}
		})
	}
}
