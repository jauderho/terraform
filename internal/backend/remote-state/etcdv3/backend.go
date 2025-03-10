package etcd

import (
	"context"

	"github.com/hashicorp/terraform/internal/backend"
	"github.com/hashicorp/terraform/internal/legacy/helper/schema"
	etcdv3 "go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"
)

const (
	endpointsKey       = "endpoints"
	usernameKey        = "username"
	usernameEnvVarName = "ETCDV3_USERNAME"
	passwordKey        = "password"
	passwordEnvVarName = "ETCDV3_PASSWORD"
	maxRequestBytesKey = "max_request_bytes"
	prefixKey          = "prefix"
	lockKey            = "lock"
	cacertPathKey      = "cacert_path"
	certPathKey        = "cert_path"
	keyPathKey         = "key_path"
)

func New() backend.Backend {
	s := &schema.Backend{
		Schema: map[string]*schema.Schema{
			endpointsKey: {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				MinItems:    1,
				Required:    true,
				Description: "Endpoints for the etcd cluster.",
			},

			usernameKey: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Username used to connect to the etcd cluster.",
				DefaultFunc: schema.EnvDefaultFunc(usernameEnvVarName, ""),
			},

			passwordKey: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Password used to connect to the etcd cluster.",
				DefaultFunc: schema.EnvDefaultFunc(passwordEnvVarName, ""),
			},

			maxRequestBytesKey: {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "The max request size to send to etcd.",
				Default:     0,
			},

			prefixKey: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "An optional prefix to be added to keys when to storing state in etcd.",
				Default:     "",
			},

			lockKey: {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Whether to lock state access.",
				Default:     true,
			},

			cacertPathKey: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The path to a PEM-encoded CA bundle with which to verify certificates of TLS-enabled etcd servers.",
				Default:     "",
			},

			certPathKey: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The path to a PEM-encoded certificate to provide to etcd for secure client identification.",
				Default:     "",
			},

			keyPathKey: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The path to a PEM-encoded key to provide to etcd for secure client identification.",
				Default:     "",
			},
		},
	}

	result := &Backend{Backend: s}
	result.Backend.ConfigureFunc = result.configure
	return result
}

type Backend struct {
	*schema.Backend

	// The fields below are set from configure.
	client *etcdv3.Client
	data   *schema.ResourceData
	lock   bool
	prefix string
}

func (b *Backend) configure(ctx context.Context) error {
	var err error
	// Grab the resource data.
	b.data = schema.FromContextBackendConfig(ctx)
	// Store the lock information.
	b.lock = b.data.Get(lockKey).(bool)
	// Store the prefix information.
	b.prefix = b.data.Get(prefixKey).(string)
	// Initialize a client to test config.
	b.client, err = b.rawClient()
	// Return err, if any.
	return err
}

func (b *Backend) rawClient() (*etcdv3.Client, error) {
	config := etcdv3.Config{}
	tlsInfo := transport.TLSInfo{}

	if v, ok := b.data.GetOk(endpointsKey); ok {
		config.Endpoints = retrieveEndpoints(v)
	}
	if v, ok := b.data.GetOk(usernameKey); ok && v.(string) != "" {
		config.Username = v.(string)
	}
	if v, ok := b.data.GetOk(passwordKey); ok && v.(string) != "" {
		config.Password = v.(string)
	}
	if v, ok := b.data.GetOk(maxRequestBytesKey); ok && v.(int) != 0 {
		config.MaxCallSendMsgSize = v.(int)
	}
	if v, ok := b.data.GetOk(cacertPathKey); ok && v.(string) != "" {
		tlsInfo.TrustedCAFile = v.(string)
	}
	if v, ok := b.data.GetOk(certPathKey); ok && v.(string) != "" {
		tlsInfo.CertFile = v.(string)
	}
	if v, ok := b.data.GetOk(keyPathKey); ok && v.(string) != "" {
		tlsInfo.KeyFile = v.(string)
	}

	if tlsCfg, err := tlsInfo.ClientConfig(); err != nil {
		return nil, err
	} else if !tlsInfo.Empty() {
		config.TLS = tlsCfg // Assign TLS configuration only if it valid and non-empty.
	}

	return etcdv3.New(config)
}

func retrieveEndpoints(v interface{}) []string {
	var endpoints []string
	list := v.([]interface{})
	for _, ep := range list {
		endpoints = append(endpoints, ep.(string))
	}
	return endpoints
}
