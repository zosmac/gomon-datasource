// Copyright © 2021 The Gomon Project.

package core

import (
	"os/user"
	"strconv"
	"sync"
)

type (
	// names defines a cache type for mapping ids to names.
	names struct {
		sync.RWMutex
		lookup func(int) string
		names  map[int]string
	}
)

var (
	// users caches user names for uids.
	users = names{
		lookup: func(id int) string {
			name := strconv.Itoa(id)
			if u, err := user.LookupId(name); err == nil {
				name = u.Name
			}
			return name
		},
		names: map[int]string{},
	}

	// groups caches group names for gids.
	groups = names{
		lookup: func(id int) string {
			name := strconv.Itoa(id)
			if g, err := user.LookupGroupId(name); err == nil {
				name = g.Name
			}
			return name
		},
		names: map[int]string{},
	}
)

// lookup retrieves and caches name for id.
func (ns *names) name(id int) string {
	ns.RLock()
	name, ok := ns.names[id]
	ns.RUnlock()
	if !ok {
		name = ns.lookup(id)
		ns.Lock()
		ns.names[id] = name
		ns.Unlock()
	}
	return name
}

// Username retrieves and caches user name for uid.
func Username(uid int) string {
	return users.name(uid)
}

// Groupname retrieves and caches group name for gid.
func Groupname(gid int) string {
	return groups.name(gid)
}
