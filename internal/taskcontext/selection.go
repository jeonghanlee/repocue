package taskcontext

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Select(declaration Declaration, root string, request Request) ([]Item, Request, error) {
	known := false
	for _, kind := range declaration.TaskKinds {
		known = known || kind == request.TaskKind
	}
	if !known || len(request.Targets) == 0 {
		return nil, request, errors.New("known task kind and at least one target are required")
	}
	directories := map[string]bool{}
	targets := map[string]bool{}
	for _, target := range request.Targets {
		if err := ValidatePath(target, true); err != nil {
			return nil, request, err
		}
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(target)))
		if err != nil && !os.IsNotExist(err) {
			return nil, request, fmt.Errorf("inspect target: %w", err)
		}
		directories[target] = target == "." || (err == nil && info.IsDir())
		targets[target] = true
	}
	request.Targets = make([]string, 0, len(targets))
	for target := range targets {
		request.Targets = append(request.Targets, target)
	}
	sort.Strings(request.Targets)
	selected := []Item{}
	for _, item := range declaration.Items {
		taskMatch := item.TaskKinds == nil
		for _, kind := range item.TaskKinds {
			taskMatch = taskMatch || kind == request.TaskKind
		}
		pathMatch := item.PathPrefixes == nil
		for _, prefix := range item.PathPrefixes {
			for _, target := range request.Targets {
				pathMatch = pathMatch || contains(prefix, target) ||
					(directories[target] && contains(target, prefix))
			}
		}
		if taskMatch && pathMatch {
			selected = append(selected, item)
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
	return selected, request, nil
}

func contains(prefix, target string) bool {
	return prefix == "." || target == prefix || strings.HasPrefix(target, prefix+"/")
}
