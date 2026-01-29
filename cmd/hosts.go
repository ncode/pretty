package cmd

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

const defaultPort = 22

type HostSpec struct {
	Host    string
	Port    int
	User    string
	PortSet bool
	UserSet bool
}

func parseHostSpec(input string) (HostSpec, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return HostSpec{}, fmt.Errorf("host is empty")
	}

	user, hostPart, hasUser, err := splitUserHost(trimmed)
	if err != nil {
		return HostSpec{}, err
	}

	if strings.HasPrefix(hostPart, "[") {
		host, portStr, err := net.SplitHostPort(hostPart)
		if err != nil {
			return HostSpec{}, fmt.Errorf("invalid host entry %q: %v", trimmed, err)
		}
		port, err := parsePort(portStr)
		if err != nil {
			return HostSpec{}, fmt.Errorf("invalid port in %q: %v", trimmed, err)
		}
		return HostSpec{Host: host, Port: port, User: user, PortSet: true, UserSet: hasUser}, nil
	}

	if hostPart == "" {
		return HostSpec{}, fmt.Errorf("host is empty")
	}

	if strings.Count(hostPart, ":") == 0 {
		return HostSpec{Host: hostPart, Port: defaultPort, User: user, UserSet: hasUser}, nil
	}
	if strings.Count(hostPart, ":") == 1 {
		parts := strings.SplitN(hostPart, ":", 2)
		if parts[0] == "" {
			return HostSpec{}, fmt.Errorf("host is empty")
		}
		port, err := parsePort(parts[1])
		if err != nil {
			return HostSpec{}, fmt.Errorf("invalid port in %q: %v", trimmed, err)
		}
		return HostSpec{Host: parts[0], Port: port, User: user, PortSet: true, UserSet: hasUser}, nil
	}

	return HostSpec{Host: hostPart, Port: defaultPort, User: user, UserSet: hasUser}, nil
}

func splitUserHost(input string) (user, host string, hasUser bool, err error) {
	at := strings.LastIndex(input, "@")
	if at == -1 {
		return "", input, false, nil
	}
	user = strings.TrimSpace(input[:at])
	host = strings.TrimSpace(input[at+1:])
	if user == "" || host == "" {
		return "", "", false, fmt.Errorf("invalid user@host")
	}
	return user, host, true, nil
}

func parsePort(port string) (int, error) {
	if port == "" {
		return 0, fmt.Errorf("port is empty")
	}
	value, err := strconv.Atoi(port)
	if err != nil {
		return 0, fmt.Errorf("port must be a number")
	}
	if value < 1 || value > 65535 {
		return 0, fmt.Errorf("port must be between 1 and 65535")
	}
	return value, nil
}

func parseHostsFile(data []byte) ([]HostSpec, error) {
	lines := strings.Split(string(data), "\n")
	specs := make([]HostSpec, 0, len(lines))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		spec, err := parseHostSpec(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid hosts file entry on line %d: %q: %v", i+1, trimmed, err)
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func parseGroupSpecs(raw interface{}, groupName string) ([]HostSpec, error) {
	if raw == nil {
		return nil, nil
	}

	value, ok := raw.(map[string]interface{})
	if !ok {
		if alt, ok := raw.(map[interface{}]interface{}); ok {
			value = make(map[string]interface{}, len(alt))
			for key, val := range alt {
				keyStr, ok := key.(string)
				if !ok {
					return nil, fmt.Errorf("invalid key type %T", key)
				}
				value[keyStr] = val
			}
		} else {
			return nil, fmt.Errorf("host group %q must be an object with hosts", groupName)
		}
	}

	hostsRaw, ok := value["hosts"]
	if !ok {
		return nil, fmt.Errorf("host group %q missing hosts", groupName)
	}
	hostsList, ok := hostsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("host group %q hosts must be a list", groupName)
	}
	if len(hostsList) == 0 {
		return nil, nil
	}

	groupUser := ""
	if userRaw, ok := value["user"]; ok {
		if userStr, ok := userRaw.(string); ok {
			groupUser = strings.TrimSpace(userStr)
		}
	}

	specs := make([]HostSpec, 0, len(hostsList))
	for i, entry := range hostsList {
		hostEntry, ok := entry.(string)
		if !ok {
			return nil, fmt.Errorf("host group %q hosts entry %d must be a string", groupName, i+1)
		}
		spec, err := parseHostSpec(hostEntry)
		if err != nil {
			return nil, fmt.Errorf("host group %q hosts entry %d: %v", groupName, i+1, err)
		}
		if !spec.UserSet && groupUser != "" {
			spec.User = groupUser
			spec.UserSet = true
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func parsePortValue(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return validatePort(v)
	case int64:
		return validatePort(int(v))
	case float64:
		if v != float64(int(v)) {
			return 0, fmt.Errorf("port must be an integer")
		}
		return validatePort(int(v))
	case string:
		return parsePort(v)
	default:
		return 0, fmt.Errorf("port must be a number")
	}
}

func validatePort(port int) (int, error) {
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port must be between 1 and 65535")
	}
	return port, nil
}

func parseArgsHosts(args []string) ([]HostSpec, error) {
	specs := make([]HostSpec, 0, len(args))
	for _, arg := range args {
		spec, err := parseHostSpec(arg)
		if err != nil {
			return nil, fmt.Errorf("invalid host %q: %v", arg, err)
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func hostDisplayName(spec HostSpec) string {
	return net.JoinHostPort(spec.Host, strconv.Itoa(spec.Port))
}
