// (c) 2019-2020, Lux Industries, Inc.
//
// This file is a derived work, based on the go-ethereum library whose original
// notices appear below.
//
// It is distributed under a license compatible with the licensing terms of the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********
// Copyright 2020 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package flags

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/luxfi/geth/internal/version"
	"github.com/luxfi/geth/params"
	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v2"
)

// usecolor defines whether the CLI help should use colored output or normal dumb
// colorless terminal formatting.
var usecolor = (isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())) && os.Getenv("TERM") != "dumb"

// NewApp creates an app with sane defaults.
func NewApp(usage string) *cli.App {
	git, _ := version.VCS()
	app := cli.NewApp()
	app.EnableBashCompletion = true
	app.Version = params.VersionWithCommit(git.Commit, git.Date)
	app.Usage = usage
	app.Copyright = "Copyright 2013-2024 The go-ethereum Authors"
	app.Before = func(ctx *cli.Context) error {
		MigrateGlobalFlags(ctx)
		return nil
	}
	return app
}

// Merge merges the given flag slices.
func Merge(groups ...[]cli.Flag) []cli.Flag {
	var ret []cli.Flag
	for _, group := range groups {
		ret = append(ret, group...)
	}
	return ret
}

var migrationApplied = map[*cli.Command]struct{}{}

// MigrateGlobalFlags makes all global flag values available in the
// context. This should be called as early as possible in app.Before.
//
// Example:
//
//	geth account new --keystore /tmp/mykeystore --lightkdf
//
// is equivalent after calling this method with:
//
//	geth --keystore /tmp/mykeystore --lightkdf account new
//
// i.e. in the subcommand Action function of 'account new', ctx.Bool("lightkdf)
// will return true even if --lightkdf is set as a global option.
//
// This function may become unnecessary when https://github.com/urfave/cli/pull/1245 is merged.
func MigrateGlobalFlags(ctx *cli.Context) {
	var iterate func(cs []*cli.Command, fn func(*cli.Command))
	iterate = func(cs []*cli.Command, fn func(*cli.Command)) {
		for _, cmd := range cs {
			if _, ok := migrationApplied[cmd]; ok {
				continue
			}
			migrationApplied[cmd] = struct{}{}
			fn(cmd)
			iterate(cmd.Subcommands, fn)
		}
	}

	// This iterates over all commands and wraps their action function.
	iterate(ctx.App.Commands, func(cmd *cli.Command) {
		if cmd.Action == nil {
			return
		}

		action := cmd.Action
		cmd.Action = func(ctx *cli.Context) error {
			doMigrateFlags(ctx)
			return action(ctx)
		}
	})
}

func doMigrateFlags(ctx *cli.Context) {
	// Figure out if there are any aliases of commands. If there are, we want
	// to ignore them when iterating over the flags.
	aliases := make(map[string]bool)
	for _, fl := range ctx.Command.Flags {
		for _, alias := range fl.Names()[1:] {
			aliases[alias] = true
		}
	}
	for _, name := range ctx.FlagNames() {
		for _, parent := range ctx.Lineage()[1:] {
			if parent.IsSet(name) {
				// When iterating across the lineage, we will be served both
				// the 'canon' and alias formats of all commands. In most cases,
				// it's fine to set it in the ctx multiple times (one for each
				// name), however, the Slice-flags are not fine.
				// The slice-flags accumulate, so if we set it once as
				// "foo" and once as alias "F", then both will be present in the slice.
				if _, isAlias := aliases[name]; isAlias {
					continue
				}
				// If it is a string-slice, we need to set it as
				// "alfa, beta, gamma" instead of "[alfa beta gamma]", in order
				// for the backing StringSlice to parse it properly.
				if result := parent.StringSlice(name); len(result) > 0 {
					ctx.Set(name, strings.Join(result, ","))
				} else {
					ctx.Set(name, parent.String(name))
				}
				break
			}
		}
	}
}

func init() {
	if usecolor {
		// Annotate all help categories with colors
		cli.AppHelpTemplate = regexp.MustCompile("[A-Z ]+:").ReplaceAllString(cli.AppHelpTemplate, "\u001B[33m$0\u001B[0m")

		// Annotate flag categories with colors (private template, so need to
		// copy-paste the entire thing here...)
		cli.AppHelpTemplate = strings.ReplaceAll(cli.AppHelpTemplate, "{{template \"visibleFlagCategoryTemplate\" .}}", "{{range .VisibleFlagCategories}}\n   {{if .Name}}\u001B[33m{{.Name}}\u001B[0m\n\n   {{end}}{{$flglen := len .Flags}}{{range $i, $e := .Flags}}{{if eq (subtract $flglen $i) 1}}{{$e}}\n{{else}}{{$e}}\n   {{end}}{{end}}{{end}}")
	}
	cli.FlagStringer = FlagString
}

// FlagString prints a single flag in help.
func FlagString(f cli.Flag) string {
	df, ok := f.(cli.DocGenerationFlag)
	if !ok {
		return ""
	}
	needsPlaceholder := df.TakesValue()
	placeholder := ""
	if needsPlaceholder {
		placeholder = "value"
	}

	namesText := cli.FlagNamePrefixer(df.Names(), placeholder)

	defaultValueString := ""
	if s := df.GetDefaultText(); s != "" {
		defaultValueString = " (default: " + s + ")"
	}
	envHint := strings.TrimSpace(cli.FlagEnvHinter(df.GetEnvVars(), ""))
	if envHint != "" {
		envHint = " (" + envHint[1:len(envHint)-1] + ")"
	}
	usage := strings.TrimSpace(df.GetUsage())
	usage = wordWrap(usage, 80)
	usage = indent(usage, 10)

	if usecolor {
		return fmt.Sprintf("\n    \u001B[32m%-35s%-35s\u001B[0m%s\n%s", namesText, defaultValueString, envHint, usage)
	} else {
		return fmt.Sprintf("\n    %-35s%-35s%s\n%s", namesText, defaultValueString, envHint, usage)
	}
}

func indent(s string, nspace int) string {
	ind := strings.Repeat(" ", nspace)
	return ind + strings.ReplaceAll(s, "\n", "\n"+ind)
}

func wordWrap(s string, width int) string {
	var (
		output     strings.Builder
		lineLength = 0
	)

	for {
		sp := strings.IndexByte(s, ' ')
		var word string
		if sp == -1 {
			word = s
		} else {
			word = s[:sp]
		}
		wlen := len(word)
		over := lineLength+wlen >= width
		if over {
			output.WriteByte('\n')
			lineLength = 0
		} else {
			if lineLength != 0 {
				output.WriteByte(' ')
				lineLength++
			}
		}

		output.WriteString(word)
		lineLength += wlen

		if sp == -1 {
			break
		}
		s = s[wlen+1:]
	}

	return output.String()
}
