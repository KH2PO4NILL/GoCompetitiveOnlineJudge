package interceptor

import (
	"errors"
	"fmt"
	"strings"
)

// InternalCallerConf describes one trusted caller and the RPC methods it may
// invoke. Services embed InternalAuthConf directly in their shared config.
type InternalCallerConf struct {
	Name    string
	Token   string
	Methods []string
}

type InternalAuthConf struct {
	Callers []InternalCallerConf
}

// ServiceClientAuthConf is the identity attached to an outgoing internal RPC.
type ServiceClientAuthConf struct {
	Name  string
	Token string
}

func (c ServiceClientAuthConf) Validate() error {
	if strings.TrimSpace(c.Name) == "" || c.Token == "" {
		return errors.New("outgoing service authentication requires name and token")
	}
	if len(strings.TrimSpace(c.Name)) > maxServiceNameLength || len(c.Token) > maxServiceTokenLength {
		return errors.New("outgoing service authentication exceeds metadata limits")
	}
	return nil
}

// BuildServiceAuthentication validates configuration and returns the two maps
// consumed by the shared service interceptor. requiredMethods ensures security-
// sensitive internal endpoints can never start without an allowlist.
func BuildServiceAuthentication(
	config InternalAuthConf,
	requiredMethods ...string,
) (map[string]string, map[string][]string, error) {
	if len(config.Callers) == 0 {
		return nil, nil, errors.New("at least one internal caller is required")
	}

	credentials := make(map[string]string, len(config.Callers))
	callers := make(map[string][]string)
	for _, caller := range config.Callers {
		name := strings.TrimSpace(caller.Name)
		if name == "" || caller.Token == "" || len(caller.Methods) == 0 {
			return nil, nil, errors.New("each internal caller requires name, token and methods")
		}
		if len(name) > maxServiceNameLength || len(caller.Token) > maxServiceTokenLength {
			return nil, nil, fmt.Errorf("internal caller %q exceeds metadata limits", name)
		}
		if _, exists := credentials[name]; exists {
			return nil, nil, fmt.Errorf("duplicate internal caller %q", name)
		}
		credentials[name] = caller.Token
		for _, rawMethod := range caller.Methods {
			method := strings.TrimSpace(rawMethod)
			if method == "" {
				return nil, nil, fmt.Errorf("internal caller %q has an empty method", name)
			}
			if !strings.HasPrefix(method, "/") {
				return nil, nil, fmt.Errorf("internal caller %q has invalid method %q", name, method)
			}
			if !containsService(callers[method], name) {
				callers[method] = append(callers[method], name)
			}
		}
	}

	for _, rawMethod := range requiredMethods {
		method := strings.TrimSpace(rawMethod)
		if method == "" {
			return nil, nil, errors.New("required internal method cannot be empty")
		}
		if len(callers[method]) == 0 {
			return nil, nil, fmt.Errorf("internal callers must cover %s", method)
		}
	}
	return credentials, callers, nil
}
