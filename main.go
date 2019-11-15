package main

import (
	"errors"
	// "flag"
	"fmt"
	"path/filepath"

	"github.com/Masterminds/glide/action"
	"github.com/Masterminds/glide/cfg"
	"github.com/Masterminds/glide/msg"
	"github.com/Masterminds/vcs"

	gpath "github.com/Masterminds/glide/path"
)

var (
	glideLockYaml = gpath.LockFile
)

func init() {
	// flag.StringVar(&glideLockYaml, "lock", gpath.LockFile, "Set a YAML configuration file")
}

type GlideLocalCheckTask struct {
	Lock *cfg.Lockfile
	Conf *cfg.Config
	Deps []*cfg.Dependency
}

func main() {
	// flag.Parse()

	// Check GOPATH existed
	action.EnsureGopath()
	msg.Info("GOPATH: %s", gpath.Gopath())
	t := GlideLocalCheckTask{}

	// Load glide.lock file
	if err := t.LoadGlideLockFile(); err != nil {
		msg.Die(err.Error())
	}

	// Only check package from glide.ymal
	if err := t.YmalDepDup(); err != nil {
		msg.Die(err.Error())
	}

	// Check local repo commit
	if err := t.CheckLocalRepoCommit(); err != nil {
		msg.Die(err.Error())
	}
}

func (t *GlideLocalCheckTask) LoadGlideLockFile() error {
	// Load glide.yaml
	t.Conf = action.EnsureConfig()

	// Load glide.lock
	var err error
	t.Lock, err = cfg.ReadLockFile(glideLockYaml)
	if err != nil {
		return err
	}

	// Check glide.lock version with glide.yaml
	hash, err := t.Conf.Hash()
	if err != nil {
		return errors.New("Could not load lockfile.")
	}

	if hash != t.Lock.Hash {
		return errors.New("Lock file may be out of date. Hash check of YAML failed. You may need to run 'update'")
	}

	return nil
}

func (t *GlideLocalCheckTask) YmalDepDup() error {

	// Load dependcies from glide.lock
	lockDeps := make(cfg.Dependencies, len(t.Lock.Imports)+len(t.Lock.DevImports))
	for k, v := range append(t.Lock.Imports, t.Lock.DevImports...) {
		lockDeps[k] = cfg.DependencyFromLock(v)
	}
	lockDeps, err := lockDeps.DeDupe()
	if err != nil {
		return err
	}

	// Load dependcies from glide.yaml
	yamlDeps := append(t.Conf.Imports, t.Conf.DevImports...)
	yamlDeps, err = yamlDeps.DeDupe()
	if err != nil {
		return err
	}

	// Remove packages in glide.lock, but not in glide.yaml.
	// Mean these packages is in the specific package vendor folder.
	// Just check the specific package commit, don't check vendor folder.
	for _, lockDep := range lockDeps {
		matched := false
		for _, yamlDep := range yamlDeps {
			if lockDep.Name == yamlDep.Name {
				matched = true
				break
			}
		}
		if matched {
			t.Deps = append(t.Deps, lockDep)
		}
	}
	return nil
}

func (t *GlideLocalCheckTask) CheckLocalRepoCommit() error {
	for _, dep := range t.Deps {
		// Get repo by dep name
		dest := fmt.Sprintf("%s/src/%s", gpath.Gopath(), filepath.ToSlash(dep.Name))
		repo, err := vcs.NewGitRepo("", dest)
		if err != nil {
			msg.Err("EX: %s", err.Error())
			continue
		}

		// Check local repo existed
		version, err := repo.Version()
		if err != nil {
			notExisted(dep.Name)
			continue
		}

		// Check local commit is equal with lock reference
		if version != dep.Reference {
			notMatched(dep.Name, dep.Reference, version)
			continue
		}

		matched(dep.Name)
	}
	return nil
}

func notMatched(name, reference, commit string) {
	mtag := msg.Color(msg.Yellow, "[Not Matched]")
	msg.Msg("%s %s (refernce: %s, commit: %s)", mtag, msg.Color(msg.Pink, name), reference, commit)
}

func notExisted(name string) {
	mtag := msg.Color(msg.Cyan, "[Not Existed]")
	msg.Msg("%s %s: no such file or directory ", mtag, msg.Color(msg.Pink, name))
}

func matched(name string) {
	mtag := msg.Color(msg.Green, "[Ok] "+name)
	msg.Msg(mtag)
}
