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
	"strings"
)

var (
	// Default target to run when none is specified
	Default = Backend

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
		return "amd64" // runtime.GOARCH
	}()

	// getExecutableName in github.com/grafana/grafana-plugin-sdk-go/build/common.go
	// expects the executable name to be appended with "_$GOOS_$GOARCH"
	backend = fmt.Sprintf("dist/%s_%s_%s", plugin, goos, goarch)

	grafanaDir = func() string {
		if dir, ok := os.LookupEnv("GRAFANA_DIR"); ok {
			return dir
		}
		return filepath.Join("..", "..", "grafana")
	}()

	confDir = filepath.Join(grafanaDir, "conf")

	pluginDir = func() string {
		if dir, ok := os.LookupEnv("GRAFANA_PLUGIN_DIR"); ok {
			return dir
		}
		return filepath.Join("..", plugin)
	}()

	credential = func() string {
		if cred, ok := os.LookupEnv("GRAFANA_CRED"); ok {
			return cred
		}
		return "-u admin:admin"
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
	if err := command("yarn", "", "install", "--fix", "--latest", "--force"); err != nil {
		return err
	}

	if err := command("yarn", "", "upgrade", "--fix", "--latest", "--force"); err != nil {
		return err
	}

	if err := command("go", "", "mod", "tidy", "-v"); err != nil {
		return err
	}

	return nil
}

// Frontend builds the web frontend of the data source.
func Frontend() error {
	if err := command("yarn", "", "build"); err != nil {
		return err
	}

	return nil
}

// Backend builds the go backend of the data source.
func Backend() error {
	if err := command("go", "", "generate", "-v", "./pkg"); err != nil {
		return err
	}

	if err := command("go", "", "build", "-v", "-o", backend, "./pkg"); err != nil {
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

// Install installs the data source plugin.
func Install() error {
	if err := command("cp", "", "./assets/custom.ini", confDir); err != nil {
		return err
	}

	if err := command("curl",
		credential,
		"-X", "DELETE",
		"http://localhost:3000/api/datasources/name/gomon-datasource",
	); err != nil {
		return err
	}

	if err := command("curl",
		credential,
		"-X", "POST",
		"-H", "Content-Type: application/json",
		"-T", "./assets/datasource.json",
		"http://localhost:3000/api/datasources",
	); err != nil {
		return err
	}

	if err := command("curl",
		credential,
		"-X", "POST",
		"-H", "Content-Type: application/json",
		"-T", "./assets/dashboard.json",
		"http://localhost:3000/api/dashboards/db",
	); err != nil {
		return err
	}

	return nil
}

// Clean up after yourself
func Clean() error {
	fmt.Println("Cleaning...")
	return os.RemoveAll("dist")
}

func command(ex, cred string, args ...string) error {
	cmd := exec.Command(ex, args...)
	fmt.Fprintf(os.Stderr, "%s\n", cmd.String())

	// For the frontend build, the webpack bundler (https://webpack.js.org) may default to a hash
	// that has been deprecated. This NODE_OPTIONS environment variable override reenables it.
	// cmd.Env = append(os.Environ(), "NODE_OPTIONS=--openssl-legacy-provider")

	if cred != "" { // add credential after echoing out command
		cmd = exec.Command(ex, append(strings.Fields(credential), args...)...)
	}

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
