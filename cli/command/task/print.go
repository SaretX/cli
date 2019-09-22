package task

import (
	"context"
	"fmt"
	"sort"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/command/formatter"
	"github.com/docker/cli/cli/command/idresolver"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/docker/api/types/swarm"
)

type tasksSortable []swarm.Task

func (t tasksSortable) Len() int {
	return len(t)
}

func (t tasksSortable) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t tasksSortable) Less(i, j int) bool {
	// Sort by service ID.
	if t[i].ServiceID != t[j].ServiceID {
		return t[i].ServiceID < t[j].ServiceID
	}

	// If same service, sort by slot.
	if t[i].Slot != t[j].Slot {
		return t[i].Slot < t[j].Slot
	}

	// If same service and slot, sort by node ID.
	// This sorting is relevant only for global services.
	if t[i].NodeID != t[j].NodeID {
		return t[i].NodeID < t[j].NodeID
	}

	// If same service, slot and node - sort by most recent.
	return t[j].Meta.CreatedAt.Before(t[i].CreatedAt)
}

// Print task information in a format.
// Besides this, command `docker node ps <node>`
// and `docker stack ps` will call this, too.
func Print(ctx context.Context, dockerCli command.Cli, tasks []swarm.Task, resolver *idresolver.IDResolver, trunc, quiet bool, format string) error {
	sort.Stable(tasksSortable(tasks))

	names := map[string]string{}
	nodes := map[string]string{}

	tasksCtx := formatter.Context{
		Output: dockerCli.Out(),
		Format: NewTaskFormat(format, quiet),
		Trunc:  trunc,
	}

	prevName := ""
	for _, task := range tasks {
		serviceName, err := resolver.Resolve(ctx, swarm.Service{}, task.ServiceID)
		if err != nil {
			return err
		}

		nodeValue, err := resolver.Resolve(ctx, swarm.Node{}, task.NodeID)
		if err != nil {
			return err
		}

		var name string
		if task.Slot != 0 {
			name = fmt.Sprintf("%v.%v", serviceName, task.Slot)
		} else {
			name = fmt.Sprintf("%v.%v", serviceName, task.NodeID)
		}

		// Indent the name if necessary
		indentedName := name
		if name == prevName {
			indentedName = fmt.Sprintf(" \\_ %s", indentedName)
		}
		prevName = name

		names[task.ID] = name
		if tasksCtx.Format.IsTable() {
			names[task.ID] = indentedName
		}
		nodes[task.ID] = nodeValue
	}

	return FormatWrite(tasksCtx, tasks, names, nodes)
}

// DefaultFormat returns the default format from the config file, or table
// format if nothing is set in the config.
func DefaultFormat(configFile *configfile.ConfigFile, quiet bool) string {
	if len(configFile.TasksFormat) > 0 && !quiet {
		return configFile.TasksFormat
	}
	return formatter.TableFormatKey
}
