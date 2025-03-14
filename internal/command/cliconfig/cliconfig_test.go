package cliconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-cmp/cmp"
)

// This is the directory where our test fixtures are.
const fixtureDir = "./testdata"

func TestLoadConfig(t *testing.T) {
	c, err := loadConfigFile(filepath.Join(fixtureDir, "config"))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	expected := &Config{
		Providers: map[string]string{
			"aws": "foo",
			"do":  "bar",
		},
	}

	if !reflect.DeepEqual(c, expected) {
		t.Fatalf("bad: %#v", c)
	}
}

func TestLoadConfig_env(t *testing.T) {
	defer os.Unsetenv("TFTEST")
	os.Setenv("TFTEST", "hello")

	c, err := loadConfigFile(filepath.Join(fixtureDir, "config-env"))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	expected := &Config{
		Providers: map[string]string{
			"aws":    "hello",
			"google": "bar",
		},
		Provisioners: map[string]string{
			"local": "hello",
		},
	}

	if !reflect.DeepEqual(c, expected) {
		t.Fatalf("bad: %#v", c)
	}
}

func TestLoadConfig_hosts(t *testing.T) {
	got, diags := loadConfigFile(filepath.Join(fixtureDir, "hosts"))
	if len(diags) != 0 {
		t.Fatalf("%s", diags.Err())
	}

	want := &Config{
		Hosts: map[string]*ConfigHost{
			"example.com": {
				Services: map[string]interface{}{
					"modules.v1": "https://example.com/",
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrong result\ngot:  %swant: %s", spew.Sdump(got), spew.Sdump(want))
	}
}

func TestLoadConfig_credentials(t *testing.T) {
	got, err := loadConfigFile(filepath.Join(fixtureDir, "credentials"))
	if err != nil {
		t.Fatal(err)
	}

	want := &Config{
		Credentials: map[string]map[string]interface{}{
			"example.com": {
				"token": "foo the bar baz",
			},
			"example.net": {
				"username": "foo",
				"password": "baz",
			},
		},
		CredentialsHelpers: map[string]*ConfigCredentialsHelper{
			"foo": {
				Args: []string{"bar", "baz"},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrong result\ngot:  %swant: %s", spew.Sdump(got), spew.Sdump(want))
	}
}

func TestConfigValidate(t *testing.T) {
	tests := map[string]struct {
		Config    *Config
		DiagCount int
	}{
		"nil": {
			nil,
			0,
		},
		"empty": {
			&Config{},
			0,
		},
		"host good": {
			&Config{
				Hosts: map[string]*ConfigHost{
					"example.com": {},
				},
			},
			0,
		},
		"host with bad hostname": {
			&Config{
				Hosts: map[string]*ConfigHost{
					"example..com": {},
				},
			},
			1, // host block has invalid hostname
		},
		"credentials good": {
			&Config{
				Credentials: map[string]map[string]interface{}{
					"example.com": {
						"token": "foo",
					},
				},
			},
			0,
		},
		"credentials with bad hostname": {
			&Config{
				Credentials: map[string]map[string]interface{}{
					"example..com": {
						"token": "foo",
					},
				},
			},
			1, // credentials block has invalid hostname
		},
		"credentials helper good": {
			&Config{
				CredentialsHelpers: map[string]*ConfigCredentialsHelper{
					"foo": {},
				},
			},
			0,
		},
		"credentials helper too many": {
			&Config{
				CredentialsHelpers: map[string]*ConfigCredentialsHelper{
					"foo": {},
					"bar": {},
				},
			},
			1, // no more than one credentials_helper block allowed
		},
		"provider_installation good none": {
			&Config{
				ProviderInstallation: nil,
			},
			0,
		},
		"provider_installation good one": {
			&Config{
				ProviderInstallation: []*ProviderInstallation{
					{},
				},
			},
			0,
		},
		"provider_installation too many": {
			&Config{
				ProviderInstallation: []*ProviderInstallation{
					{},
					{},
				},
			},
			1, // no more than one provider_installation block allowed
		},
		"plugin_cache_dir does not exist": {
			&Config{
				PluginCacheDir: "fake",
			},
			1, // The specified plugin cache dir %s cannot be opened
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			diags := test.Config.Validate()
			if len(diags) != test.DiagCount {
				t.Errorf("wrong number of diagnostics %d; want %d", len(diags), test.DiagCount)
				for _, diag := range diags {
					t.Logf("- %#v", diag.Description())
				}
			}
		})
	}
}

func TestConfig_Merge(t *testing.T) {
	c1 := &Config{
		Providers: map[string]string{
			"foo": "bar",
			"bar": "blah",
		},
		Provisioners: map[string]string{
			"local":  "local",
			"remote": "bad",
		},
		Hosts: map[string]*ConfigHost{
			"example.com": {
				Services: map[string]interface{}{
					"modules.v1": "http://example.com/",
				},
			},
		},
		Credentials: map[string]map[string]interface{}{
			"foo": {
				"bar": "baz",
			},
		},
		CredentialsHelpers: map[string]*ConfigCredentialsHelper{
			"buz": {},
		},
		ProviderInstallation: []*ProviderInstallation{
			{
				Methods: []*ProviderInstallationMethod{
					{Location: ProviderInstallationFilesystemMirror("a")},
					{Location: ProviderInstallationFilesystemMirror("b")},
				},
			},
			{
				Methods: []*ProviderInstallationMethod{
					{Location: ProviderInstallationFilesystemMirror("c")},
				},
			},
		},
	}

	c2 := &Config{
		Providers: map[string]string{
			"bar": "baz",
			"baz": "what",
		},
		Provisioners: map[string]string{
			"remote": "remote",
		},
		Hosts: map[string]*ConfigHost{
			"example.net": {
				Services: map[string]interface{}{
					"modules.v1": "https://example.net/",
				},
			},
		},
		Credentials: map[string]map[string]interface{}{
			"fee": {
				"bur": "bez",
			},
		},
		CredentialsHelpers: map[string]*ConfigCredentialsHelper{
			"biz": {},
		},
		ProviderInstallation: []*ProviderInstallation{
			{
				Methods: []*ProviderInstallationMethod{
					{Location: ProviderInstallationFilesystemMirror("d")},
				},
			},
		},
	}

	expected := &Config{
		Providers: map[string]string{
			"foo": "bar",
			"bar": "baz",
			"baz": "what",
		},
		Provisioners: map[string]string{
			"local":  "local",
			"remote": "remote",
		},
		Hosts: map[string]*ConfigHost{
			"example.com": {
				Services: map[string]interface{}{
					"modules.v1": "http://example.com/",
				},
			},
			"example.net": {
				Services: map[string]interface{}{
					"modules.v1": "https://example.net/",
				},
			},
		},
		Credentials: map[string]map[string]interface{}{
			"foo": {
				"bar": "baz",
			},
			"fee": {
				"bur": "bez",
			},
		},
		CredentialsHelpers: map[string]*ConfigCredentialsHelper{
			"buz": {},
			"biz": {},
		},
		ProviderInstallation: []*ProviderInstallation{
			{
				Methods: []*ProviderInstallationMethod{
					{Location: ProviderInstallationFilesystemMirror("a")},
					{Location: ProviderInstallationFilesystemMirror("b")},
				},
			},
			{
				Methods: []*ProviderInstallationMethod{
					{Location: ProviderInstallationFilesystemMirror("c")},
				},
			},
			{
				Methods: []*ProviderInstallationMethod{
					{Location: ProviderInstallationFilesystemMirror("d")},
				},
			},
		},
	}

	actual := c1.Merge(c2)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Fatalf("wrong result\n%s", diff)
	}
}

func TestConfig_Merge_disableCheckpoint(t *testing.T) {
	c1 := &Config{
		DisableCheckpoint: true,
	}

	c2 := &Config{}

	expected := &Config{
		Providers:         map[string]string{},
		Provisioners:      map[string]string{},
		DisableCheckpoint: true,
	}

	actual := c1.Merge(c2)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %#v", actual)
	}
}

func TestConfig_Merge_disableCheckpointSignature(t *testing.T) {
	c1 := &Config{
		DisableCheckpointSignature: true,
	}

	c2 := &Config{}

	expected := &Config{
		Providers:                  map[string]string{},
		Provisioners:               map[string]string{},
		DisableCheckpointSignature: true,
	}

	actual := c1.Merge(c2)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %#v", actual)
	}
}
