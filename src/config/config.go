package config

import (
	"LocalSignTools/src/builders"
	"crypto/rand"
	"encoding/hex"
	"github.com/ViRb3/koanf-extra/env"
	"github.com/knadh/koanf"
	kyaml "github.com/knadh/koanf/parsers/yaml"
	kfile "github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

type BasicAuth struct {
	Enable   bool   `yaml:"enable"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// Builder contains configuration for all available builders.
// For LocalSignTools, only the integrated builder is supported.
type Builder struct {
	Integrated builders.IntegratedData `yaml:"integrated"`
}

func (b *Builder) MakeEnabled() map[string]builders.Builder {
	results := map[string]builders.Builder{}
	if b.Integrated.Enable {
		results["Integrated"] = builders.MakeIntegrated(&b.Integrated)
	}
	return results
}

type File struct {
	Builder             Builder   `yaml:"builder"`
	ServerUrl           string    `yaml:"server_url"`
	RedirectHttps       bool      `yaml:"redirect_https"`
	SaveDir             string    `yaml:"save_dir"`
	CleanupIntervalMins uint64    `yaml:"cleanup_interval_mins"`
	SignTimeoutMins     uint64    `yaml:"sign_timeout_mins"`
	BasicAuth           BasicAuth `yaml:"basic_auth"`
	BuilderKey          string    `yaml:"builder_key,omitempty"`
}

func createDefaultFile() *File {
	return &File{
		Builder: Builder{
			Integrated: builders.IntegratedData{
				Enable:        true,
				SignFilesDir:  "./builder",
				Entrypoint:    "sign.py",
				JobTimeoutMin: 15,
			},
		},
		ServerUrl:           "http://localhost:8080",
		RedirectHttps:       false,
		SaveDir:             "data",
		SignTimeoutMins:     60,
		CleanupIntervalMins: 5,
		BasicAuth: BasicAuth{
			Enable:   false,
			Username: "admin",
			Password: "admin",
		},
	}
}

type EnvProfile struct {
	Name        string `yaml:"name"`
	ProvBase64  string `yaml:"prov_base64"`
	CertPass    string `yaml:"cert_pass"`
	CertBase64  string `yaml:"cert_base64"`
	AccountName string `yaml:"account_name"`
	AccountPass string `yaml:"account_pass"`
}

type ProfileBox struct {
	EnvProfile `yaml:"profile"`
}

type Config struct {
	Builder    map[string]builders.Builder
	BuilderKey string
	*File
	EnvProfile *EnvProfile
}

var Current Config

func Load(fileName string) {
	allowedExts := []string{".yml", ".yaml"}
	if !isAllowedExt(allowedExts, fileName) {
		log.Fatal().Msgf("config extension not allowed: %v\n", allowedExts)
	}
	mapDelim := '.'
	fileConfig, err := getFile(mapDelim, fileName)
	if err != nil {
		log.Fatal().Err(err).Msg("get config")
	}
	builderMap := fileConfig.Builder.MakeEnabled()
	if len(builderMap) < 1 {
		log.Fatal().Msg("init: no builders defined")
	}
	
	// Generate or load builder key
	var builderKey string
	if fileConfig.BuilderKey != "" {
		// Use existing key from config file
		builderKey = fileConfig.BuilderKey
		log.Info().Msg("using existing builder key from config")
	} else {
		// Generate new key
		keyBytes := make([]byte, 32)
		if _, err := rand.Read(keyBytes); err != nil {
			log.Fatal().Err(err).Msg("init: error generating builder key")
		}
		builderKey = hex.EncodeToString(keyBytes)
		// Save to config file
		fileConfig.BuilderKey = builderKey
		if err := saveFile(fileName, fileConfig); err != nil {
			log.Warn().Err(err).Msg("failed to save builder key to config file")
		} else {
			log.Info().Msg("generated and saved new builder key to config file")
		}
	}
	
	profile, err := getProfileFromEnv(mapDelim)
	if err != nil {
		log.Fatal().Err(err).Msg("init: error checking for signing profile from envvars")
	}
	Current = Config{
		Builder:    builderMap,
		BuilderKey: builderKey,
		File:       fileConfig,
		EnvProfile: profile,
	}
}

// Loads a single signing profile entirely from environment variables.
// Intended for use with Heroku without persistent storage.
func getProfileFromEnv(mapDelim rune) (*EnvProfile, error) {
	k := koanf.New(string(mapDelim))
	if err := k.Load(structs.Provider(ProfileBox{}, "yaml"), nil); err != nil {
		return nil, errors.WithMessage(err, "load default")
	}
	if err := k.Load(env.Provider(k, "", "_", func(s string) string {
		return strings.ToLower(s)
	}), nil); err != nil {
		return nil, errors.WithMessage(err, "load envvars")
	}
	profile := EnvProfile{}
	if err := k.UnmarshalWithConf("profile", &profile, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		return nil, errors.WithMessage(err, "unmarshal")
	}
	return &profile, nil
}

func isAllowedExt(allowedExts []string, fileName string) bool {
	fileExt := filepath.Ext(fileName)
	for _, ext := range allowedExts {
		if fileExt == ext {
			return true
		}
	}
	return false
}

func getFile(mapDelim rune, fileName string) (*File, error) {
	k := koanf.New(string(mapDelim))
	if err := k.Load(structs.Provider(createDefaultFile(), "yaml"), nil); err != nil {
		return nil, errors.WithMessage(err, "load default")
	}
	if err := k.Load(kfile.Provider(fileName), kyaml.Parser()); os.IsNotExist(err) {
		log.Info().Str("name", fileName).Msg("creating config file")
	} else if err != nil {
		return nil, errors.WithMessage(err, "load existing")
	}
	// Note: Environment variables with the same path may overwrite each other.
	// For example, PROFILE_CERT_NAME="bar" and PROFILE_CERT="foo" both map to "profile.cert".
	// This is a limitation of the current configuration system.
	if err := k.Load(env.Provider(k, "", "_", func(s string) string {
		return strings.ToLower(s)
	}), nil); err != nil {
		return nil, errors.WithMessage(err, "load envvars")
	}
	fileConfig := File{}
	if err := k.UnmarshalWithConf("", &fileConfig, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		return nil, errors.WithMessage(err, "unmarshal")
	}
	// Only save config file if it doesn't exist (initial creation)
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		if err := saveFile(fileName, &fileConfig); err != nil {
			return nil, errors.WithMessage(err, "save initial config")
		}
	}
	return &fileConfig, nil
}

// SetBuilderSecrets sets the secrets for a builder using the current configuration.
// This is a utility function to avoid code duplication.
func SetBuilderSecrets(builder builders.Builder) error {
	secrets := map[string]string{
		"SECRET_KEY": Current.BuilderKey,
		"SECRET_URL": Current.ServerUrl,
	}
	return errors.WithMessage(builder.SetSecrets(secrets), "set builder secrets")
}

// saveFile saves the config file to disk
func saveFile(fileName string, fileConfig *File) error {
	file, err := os.Create(fileName)
	if err != nil {
		return errors.WithMessage(err, "create")
	}
	defer file.Close()
	if err := yaml.NewEncoder(file).Encode(fileConfig); err != nil {
		return errors.WithMessage(err, "write")
	}
	return nil
}
