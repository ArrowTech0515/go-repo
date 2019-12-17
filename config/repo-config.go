package config

import (
	"os"
	"path/filepath"

	log "github.com/jiangxin/multi-log"
)

const (
	// DefaultGitRepoConfigFile is default git-repo config file
	DefaultGitRepoConfigFile = "config"
)

var (
	gitRepoConfigExample = `
# Example git-repo config file, generated by git-repo.
# DO NOT edit this file! Any modification will be overwritten.
#

# Set console verbosity. 1: show info, 2: show debug, 3: show trace
#verbose: 0

# LogLvel for logging to file
#loglevel: warning

# LogRotate defines max size of the logfile
#logrotate: 20M

# LogFile defines where to save log
#logfile:
`
)

// InstallRepoConfig installs default git-repo config example file.
func InstallRepoConfig() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}
	filename := filepath.Join(configDir, DefaultGitRepoConfigFile+".yml.example")
	if err != nil {
		return err
	}

	fi, err := os.Stat(filename)
	if err == nil {
		if fi.Size() == int64(len(gitRepoConfigExample)) {
			return nil
		}
	}

	log.Debugf("install git-repo config file: %s", filename)

	dir := filepath.Dir(filename)
	if _, err := os.Stat(dir); err != nil {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}

	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(gitRepoConfigExample)
	return err
}
