//go:build mage

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

var (
	// Default target to run when none is specified
	Default = Backend

	devDir = func() string {
		dir := filepath.Join(os.Getenv("HOME"), "Developer")
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			return dir
		}
		return os.Getenv("HOME")
	}()

	plugin = "gomon-datasource"

	goos = func() string {
		if goos, ok := os.LookupEnv("GOOS"); ok {
			return goos
		}
		return runtime.GOOS
	}()

	goarch = func() string {
		if goarch, ok := os.LookupEnv("GOARCH"); ok {
			return goarch
		}
		return runtime.GOARCH
	}()

	// getExecutableName in github.com/grafana/grafana-plugin-sdk-go/build/common.go
	// expects the executable name to be appended with "_$GOOS_$GOARCH"
	backend = fmt.Sprintf("dist/%s_%s_%s", plugin, goos, goarch)

	grafanaDir = func() string {
		if dir, ok := os.LookupEnv("GRAFANA_DIR"); ok {
			return dir
		}

		if glob, err := filepath.Glob(filepath.Join(devDir, "grafana-v*")); err == nil {
			if len(glob) == 1 {
				return glob[0]
				// else do some semantic version sorting
			}
		}

		return filepath.Join("..", "grafana")
	}()

	confDir = filepath.Join(grafanaDir, "conf")

	pluginDir = func() string {
		if dir, ok := os.LookupEnv("PLUGIN_DIR"); ok {
			return dir
		}
		return filepath.Join(devDir, "plugins", "zosmac-gomon-datasource")
	}()

	verbose = func() bool {
		if verb, ok := os.LookupEnv("MAGEFILE_VERBOSE"); ok && verb == "1" { // also set by -v
			return true
		}
		return false
	}()
)

// Upgrade refreshes dependencies.
func Upgrade() error {
	if err := command("npm", "update", "--save"); err != nil {
		return err
	}

	command("rm", "-rf", "node_modules")

	if err := command("npm", "clean-install"); err != nil {
		return err
	}

	return nil
}

// Frontend builds the web frontend of the data source.
func Frontend() error {
	if err := command("npm", "run", "build"); err != nil {
		return err
	}

	// copy assets including images for README.md
	if err := command("cp", "-R", "assets", "dist/"); err != nil {
		return err
	}

	return nil
}

// Backend builds the go backend of the data source.
func Backend() error {
	if err := command("go", "generate", "-v", "./pkg"); err != nil {
		return err
	}

	if err := command("go", "build", "-v", "-o", backend, "./pkg"); err != nil {
		return err
	}

	return nil
}

// BuildAll builds both the frontend and backend.
func BuildAll() error {
	if err := Frontend(); err != nil {
		return err
	}

	if err := Backend(); err != nil {
		return err
	}

	return nil
}

// Clean up after yourself
func Clean() error {
	fmt.Println("Cleaning...")
	return os.RemoveAll("dist")
}

func command(ex string, args ...string) error {
	cmd := exec.Command(ex, args...)
	fmt.Fprintf(os.Stderr, "%s\n", cmd.String())

	// For the frontend build, the webpack bundler (https://webpack.js.org) may default to a hash
	// that has been deprecated. This NODE_OPTIONS environment variable override reenables it.
	// cmd.Env = append(os.Environ(), "NODE_OPTIONS=--openssl-legacy-provider")

	o := &bytes.Buffer{}
	e := &bytes.Buffer{}
	cmd.Stdout = o
	cmd.Stderr = e
	err := cmd.Run()
	if verbose {
		io.Copy(os.Stdout, o)
		io.Copy(os.Stderr, e)
	}

	return err
}
