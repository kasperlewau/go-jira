package main

import (
	"bytes"
	"fmt"
	"github.com/Netflix-Skunkworks/go-jira/jira/cli"
	"github.com/coryb/optigo"
	"github.com/op/go-logging"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

var log = logging.MustGetLogger("jira")
var format = "%{color}%{time:2006-01-02T15:04:05.000Z07:00} %{level:-5s} [%{shortfile}]%{color:reset} %{message}"

func main() {
	logBackend := logging.NewLogBackend(os.Stderr, "", 0)
	logging.SetBackend(
		logging.NewBackendFormatter(
			logBackend,
			logging.MustStringFormatter(format),
		),
	)
	logging.SetLevel(logging.NOTICE, "")

	user := os.Getenv("USER")
	home := os.Getenv("HOME")
	defaultMaxResults := 500

	usage := func(ok bool) {
		printer := fmt.Printf
		if !ok {
			printer = func(format string, args ...interface{}) (int, error) {
				return fmt.Fprintf(os.Stderr, format, args...)
			}
			defer func() {
				os.Exit(1)
			}()
		} else {
			defer func() {
				os.Exit(0)
			}()
		}
		output := fmt.Sprintf(`
Usage:
  jira (ls|list) ( [-q JQL] | [-p PROJECT] [-c COMPONENT] [-a ASSIGNEE] [-i ISSUETYPE] [-w WATCHER] [-r REPORTER]) [-f FIELDS] [-s ORDER] [--max_results MAX_RESULTS]
  jira view ISSUE
  jira edit ISSUE [--noedit] [-m COMMENT] [-o KEY=VAL]... 
  jira create [--noedit] [-p PROJECT] [-i ISSUETYPE] [-o KEY=VAL]...
  jira DUPLICATE dups ISSUE
  jira BLOCKER blocks ISSUE
  jira watch ISSUE [-w WATCHER]
  jira (trans|transition) TRANSITION ISSUE [-m COMMENT] [-o KEY=VAL] [--noedit]
  jira ack ISSUE [-m COMMENT] [-o KEY=VAL] [--edit] 
  jira close ISSUE [-m COMMENT] [-o KEY=VAL] [--edit]
  jira resolve ISSUE [-m COMMENT] [-o KEY=VAL] [--edit]
  jira reopen ISSUE [-m COMMENT] [-o KEY=VAL] [--edit]
  jira start ISSUE [-m COMMENT] [-o KEY=VAL] [--edit]
  jira stop ISSUE [-m COMMENT] [-o KEY=VAL] [--edit]
  jira comment ISSUE [-m COMMENT]
  jira take ISSUE
  jira (assign|give) ISSUE ASSIGNEE
  jira fields
  jira issuelinktypes
  jira transmeta ISSUE
  jira editmeta ISSUE
  jira issuetypes [-p PROJECT] 
  jira createmeta [-p PROJECT] [-i ISSUETYPE] 
  jira transitions ISSUE
  jira export-templates [-d DIR] [-t template]
  jira (b|browse) ISSUE
  jira login
  jira ISSUE
 
General Options:
  -e --endpoint=URI   URI to use for jira
  -h --help           Show this usage
  -t --template=FILE  Template file to use for output/editing
  -u --user=USER      Username to use for authenticaion (default: %s)
  -v --verbose        Increase output logging

Command Options:
  -a --assignee=USER        Username assigned the issue
  -b --browse               Open your browser to the Jira issue
  -c --component=COMPONENT  Component to Search for
  -d --directory=DIR        Directory to export templates to (default: %s)
  -f --queryfields=FIELDS   Fields that are used in "list" template: (default: summary,created,priority,status,reporter,assignee)
  -i --issuetype=ISSUETYPE  Jira Issue Type (default: Bug)
  -m --comment=COMMENT      Comment message for transition
  -o --override=KEY=VAL     Set custom key/value pairs
  -p --project=PROJECT      Project to Search for
  -q --query=JQL            Jira Query Language expression for the search
  -r --reporter=USER        Reporter to search for
  -s --sort=ORDER           For list operations, sort issues (default: priority asc, created)
  -w --watcher=USER         Watcher to add to issue (default: %s)
                            or Watcher to search for
  --max_results=VAL         Maximum number of results to return in query (default: %d)
`, user, fmt.Sprintf("%s/.jira.d/templates", home), user, defaultMaxResults)
		printer(output)
	}

	jiraCommands := map[string]string{
		"list":             "list",
		"ls":               "list",
		"view":             "view",
		"edit":             "edit",
		"create":           "create",
		"dups":             "dups",
		"blocks":           "blocks",
		"watch":            "watch",
		"trans":            "transition",
		"transition":       "transition",
		"ack":              "acknowledge",
		"acknowledge":      "acknowledge",
		"close":            "close",
		"resolve":          "resolve",
		"reopen":           "reopen",
		"start":            "start",
		"stop":             "stop",
		"comment":          "comment",
		"take":             "take",
		"assign":           "assign",
		"give":             "assign",
		"fields":           "fields",
		"issuelinktypes":   "issuelinktypes",
		"transmeta":        "transmeta",
		"editmeta":         "editmeta",
		"issuetypes":       "issuetypes",
		"createmeta":       "createmeta",
		"transitions":      "transitions",
		"export-templates": "export-templates",
		"browse":           "browse",
		"login":            "login",
	}

	opts := map[string]interface{}{
		"user":        user,
		"issuetype":   "Bug",
		"watcher":     user,
		"queryfields": "summary,created,priority,status,reporter,assignee",
		"directory":   fmt.Sprintf("%s/.jira.d/templates", home),
		"sort":        "priority asc, created",
		"max_results": defaultMaxResults,
	}
	overrides := make(map[string]string)

	setopt := func(name string, value interface{}) {
		opts[name] = value
	}

	op := optigo.NewDirectAssignParser(map[string]interface{}{
		"h|help": usage,
		"v|verbose+": func() {
			logging.SetLevel(logging.GetLevel("")+1, "")
		},
		"dryrun":                setopt,
		"b|browse":              setopt,
		"editor=s":              setopt,
		"u|user=s":              setopt,
		"endpoint=s":            setopt,
		"t|template=s":          setopt,
		"q|query=s":             setopt,
		"p|project=s":           setopt,
		"c|component=s":         setopt,
		"a|assignee=s":          setopt,
		"i|issuetype=s":         setopt,
		"w|watcher=s":           setopt,
		"r|reporter=s":          setopt,
		"f|queryfields=s":       setopt,
		"s|sort=s":              setopt,
		"l|limit|max_results=i": setopt,
		"o|override=s%":         &overrides,
		"noedit":                setopt,
		"edit":                  setopt,
		"m|comment=s":           setopt,
		"d|dir|directory=s":     setopt,
	})

	if err := op.ProcessAll(os.Args[1:]); err != nil {
		log.Error("%s", err)
		usage(false)
	}
	args := op.Args
	opts["overrides"] = overrides

	command := "view"
	if len(args) > 0 {
		if alias, ok := jiraCommands[args[0]]; ok {
			command = alias
			args = args[1:]
		} else if len(args) > 1 {
			// look at second arg for "dups" and "blocks" commands
			if alias, ok := jiraCommands[args[1]]; ok {
				command = alias
				args = append(args[:1], args[2:]...)
			}
		}
	}

	os.Setenv("JIRA_OPERATION", command)
	loadConfigs(opts)

	if _, ok := opts["endpoint"]; !ok {
		log.Error("endpoint option required.  Either use --endpoint or set a enpoint option in your ~/.jira.d/config.yml file")
		os.Exit(1)
	}

	c := cli.New(opts)

	log.Debug("opts: %s", opts)

	setEditing := func(dflt bool) {
		if dflt {
			if val, ok := opts["noedit"].(bool); ok && val {
				opts["edit"] = false
			} else {
				opts["edit"] = true
			}
		} else {
			if val, ok := opts["edit"].(bool); ok && !val {
				opts["edit"] = false
			}
		}
	}

	var err error
	switch command {
	case "login":
		err = c.CmdLogin()
	case "fields":
		err = c.CmdFields()
	case "list":
		err = c.CmdList()
	case "edit":
		setEditing(true)
		err = c.CmdEdit(args[0])
	case "editmeta":
		err = c.CmdEditMeta(args[0])
	case "transmeta":
		err = c.CmdTransitionMeta(args[0])
	case "issuelinktypes":
		err = c.CmdIssueLinkTypes()
	case "issuetypes":
		err = c.CmdIssueTypes()
	case "createmeta":
		err = c.CmdCreateMeta()
	case "create":
		setEditing(true)
		err = c.CmdCreate()
	case "transitions":
		err = c.CmdTransitions(args[0])
	case "blocks":
		err = c.CmdBlocks(args[0], args[1])
	case "dups":
		if err = c.CmdDups(args[0], args[1]); err == nil {
			opts["resolution"] = "Duplicate"
			err = c.CmdTransition(args[0], "close")
		}
	case "watch":
		err = c.CmdWatch(args[0])
	case "transition":
		setEditing(true)
		err = c.CmdTransition(args[0], args[1])
	case "close":
		setEditing(false)
		err = c.CmdTransition(args[0], "close")
	case "acknowledge":
		setEditing(false)
		err = c.CmdTransition(args[0], "acknowledge")
	case "reopen":
		setEditing(false)
		err = c.CmdTransition(args[0], "reopen")
	case "resolve":
		setEditing(false)
		err = c.CmdTransition(args[0], "resolve")
	case "start":
		setEditing(false)
		err = c.CmdTransition(args[0], "start")
	case "stop":
		setEditing(false)
		err = c.CmdTransition(args[0], "stop")
	case "comment":
		setEditing(true)
		err = c.CmdComment(args[0])
	case "take":
		err = c.CmdAssign(args[0], opts["user"].(string))
	case "browse":
		opts["browse"] = true
		err = c.Browse(args[0])
	case "export-tempaltes":
		err = c.CmdExportTemplates()
	case "assign":
		err = c.CmdAssign(args[0], args[1])
	default:
		err = c.CmdView(args[0])
	}

	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func parseYaml(file string, opts map[string]interface{}) {
	if fh, err := ioutil.ReadFile(file); err == nil {
		log.Debug("Found Config file: %s", file)
		yaml.Unmarshal(fh, &opts)
	}
}

func populateEnv(opts map[string]interface{}) {
	for k, v := range opts {
		envName := fmt.Sprintf("JIRA_%s", strings.ToUpper(k))
		var val string
		switch t := v.(type) {
		case string:
			val = t
		case int, int8, int16, int32, int64:
			val = fmt.Sprintf("%d", t)
		case float32, float64:
			val = fmt.Sprintf("%f", t)
		case bool:
			val = fmt.Sprintf("%t", t)
		default:
			val = fmt.Sprintf("%v", t)
		}
		os.Setenv(envName, val)
	}
}

func loadConfigs(opts map[string]interface{}) {
	populateEnv(opts)
	paths := cli.FindParentPaths(".jira.d/config.yml")
	// prepend
	paths = append([]string{"/etc/jira-cli.yml"}, paths...)

	for _, file := range paths {
		if stat, err := os.Stat(file); err == nil {
			// check to see if config file is exectuable
			if stat.Mode()&0111 == 0 {
				parseYaml(file, opts)
			} else {
				log.Debug("Found Executable Config file: %s", file)
				// it is executable, so run it and try to parse the output
				cmd := exec.Command(file)
				stdout := bytes.NewBufferString("")
				cmd.Stdout = stdout
				cmd.Stderr = bytes.NewBufferString("")
				if err := cmd.Run(); err != nil {
					log.Error("%s is exectuable, but it failed to execute: %s\n%s", file, err, cmd.Stderr)
					os.Exit(1)
				}
				yaml.Unmarshal(stdout.Bytes(), &opts)
				populateEnv(opts)
			}
		}
	}
}
