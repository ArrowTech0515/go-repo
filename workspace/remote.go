package workspace

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"code.alibaba-inc.com/force/git-repo/config"
	"code.alibaba-inc.com/force/git-repo/manifest"
	"code.alibaba-inc.com/force/git-repo/project"
	"github.com/jiangxin/multi-log"
)

const (
	remoteCallTimeout = 10
)

var (
	httpClient *http.Client
)

// LoadRemotes calls remote API to get server type and other info
func (v *WorkSpace) LoadRemotes() error {
	if v.Manifest == nil || v.Manifest.Remotes == nil {
		return nil
	}

	cfg := v.ManifestProject.Config()
	for _, r := range v.Manifest.Remotes {
		t := cfg.Get(fmt.Sprintf(config.CfgManifestRemoteType, r.Name))
		if t != "" {
			sshInfo := cfg.Get(fmt.Sprintf(config.CfgManifestRemoteSSHInfo, r.Name))
			remote, err := project.NewRemote(&r, t, sshInfo)
			if err != nil {
				return err
			}
			v.RemoteMap[r.Name] = remote
		} else {
			remote, err := v.loadRemote(&r)
			if err != nil {
				return err
			}
			v.RemoteMap[r.Name] = remote
			// TODO: set git config
		}
	}

	for i := range v.Projects {
		name := v.Projects[i].ManifestRemote.Name
		if name == "" {
			log.Warnf("empty remote for project '%s'",
				v.Projects[i].Name)
			continue
		}
		v.Projects[i].Remote = v.RemoteMap[name]
	}

	return nil
}

func getHTTPClient() *http.Client {
	if httpClient != nil {
		return httpClient
	}

	skipSSLVerify := config.NoCertChecks()

	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   remoteCallTimeout * time.Second,
			KeepAlive: remoteCallTimeout * time.Second,
		}).DialContext,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: skipSSLVerify},
		TLSHandshakeTimeout:   remoteCallTimeout * time.Second,
		ResponseHeaderTimeout: remoteCallTimeout * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          10,
		IdleConnTimeout:       remoteCallTimeout * time.Second,
		DisableCompression:    true,
	}

	httpClient = &http.Client{Transport: tr}

	return httpClient
}

func (v WorkSpace) loadRemote(r *manifest.Remote) (project.Remote, error) {
	if _, ok := v.RemoteMap[r.Name]; ok {
		return v.RemoteMap[r.Name], nil
	}

	return loadRemote(r)
}

func loadRemote(r *manifest.Remote) (project.Remote, error) {
	var (
		remoteType = r.Type
	)

	u := r.Review
	if strings.HasSuffix(u, "/") {
		u = strings.TrimSuffix(u, "/")
	}

	if u == "" {
		return &project.UnknownRemote{
			Remote: *r,
		}, nil
	}

	if strings.HasPrefix(u, "persistent-") {
		u = u[len("persistent-"):]
	}
	proto := strings.SplitN(u, ":", 2)[0]
	if proto != "http" && proto != "https" && proto != "sso" && proto != "ssh" {
		u = "http://" + u
	}
	if strings.HasSuffix(strings.ToLower(u), "/gerrit") {
		u = u[0 : len(u)-len("/Gerrit")]
		remoteType = config.RemoteTypeGerrit
	}
	if strings.HasSuffix(strings.ToLower(u), "/agit") {
		u = u[0 : len(u)-len("/AGit")]
		remoteType = config.RemoteTypeAGit
	}
	if strings.HasSuffix(u, "/ssh_info") {
		u = strings.TrimSuffix(u, "/ssh_info")
	}

	sshInfo := os.Getenv("REPO_HOST_PORT_INFO")
	if sshInfo != "" {
		if remoteType == "" {
			remoteType = config.RemoteTypeGerrit
		}
		return project.NewRemote(r, remoteType, sshInfo)
	}

	if strings.HasPrefix(u, "sso:") ||
		strings.HasPrefix(u, "ssh:") {
		if remoteType == "" {
			remoteType = config.RemoteTypeGerrit
		}
		return project.NewRemote(r, remoteType, "")
	}

	if os.Getenv("REPO_IGNORE_SSH_INFO") != "" {
		if remoteType == "" {
			remoteType = config.RemoteTypeGerrit
		}
		return project.NewRemote(r, remoteType, "")
	}

	infoURL := u + "/ssh_info"
	log.Debugf("start checking ssh_info from %s", infoURL)

	req, err := http.NewRequest("GET", infoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	client := getHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// Successful status code maybe 200, 201.
	if resp.StatusCode >= 300 {
		log.Errorf("bad ssh_info respose, status: %d", resp.StatusCode)
		if remoteType == "" {
			remoteType = config.RemoteTypeUnknown
		}
		return project.NewRemote(r, remoteType, "")
	}

	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')

	if err != nil && err != io.EOF {
		return nil, err
	}

	// If `info` contains '<', we assume the server gave us some sort
	// of HTML response back, like maybe a login page.
	//
	// Assume HTTP if SSH is not enabled or ssh_info doesn't look right.
	if line == "NOT_AVAILABLE" || strings.HasPrefix(line, "<") {
		if remoteType == "" {
			if line == "NOT_AVAILABLE" {
				remoteType = config.RemoteTypeGerrit
			} else {
				remoteType = config.RemoteTypeUnknown
			}
		}
		return project.NewRemote(r, remoteType, "")
	}

	buf := line
	n := 0
	for {
		line, err = reader.ReadString('\n')
		buf += line
		n++
		if err != nil || n > 10 {
			break
		}
	}

	return project.NewRemote(r, remoteType, buf)
}
