package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

func TestMongoYAMLFieldsBuildDriverURI(t *testing.T) {
	t.Parallel()

	raw := `
mongo:
  uri: mongodb://stale.example:27018/otherdb
  authSource: admin
  db: f_ccg_video
  host: 192.168.1.3
  maxPool: 50
  password: "123456"
  port: 27017
  timeoutSec: 5
  username: admin
`
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(raw)); err != nil {
		t.Fatalf("read config: %v", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Mongo == nil {
		t.Fatal("mongo config is nil")
	}
	if cfg.Mongo.Host != "192.168.1.3" || cfg.Mongo.Port != 27017 || cfg.Mongo.DBName != "f_ccg_video" {
		t.Fatalf("mongo address = %+v", cfg.Mongo)
	}
	if cfg.Mongo.Username != "admin" || cfg.Mongo.Password != "123456" || cfg.Mongo.AuthSource != "admin" {
		t.Fatalf("mongo auth = %+v", cfg.Mongo)
	}
	if cfg.Mongo.TimeoutSec != 5 || cfg.Mongo.MaxPool != 50 {
		t.Fatalf("mongo pool = %+v", cfg.Mongo)
	}

	uri := cfg.Mongo.GetURI()
	if strings.Contains(uri, "stale.example") || strings.Contains(uri, "otherdb") {
		t.Fatalf("GetURI = %q", uri)
	}

	parsed, err := connstring.Parse(uri)
	if err != nil {
		t.Fatalf("parse uri: %v", err)
	}
	if err = options.Client().ApplyURI(uri).Validate(); err != nil {
		t.Fatalf("ApplyURI: %v", err)
	}
	if parsed.Username != "admin" || parsed.Password != "123456" {
		t.Fatalf("user=%q password=%q", parsed.Username, parsed.Password)
	}
	if len(parsed.Hosts) != 1 || parsed.Hosts[0] != "192.168.1.3:27017" {
		t.Fatalf("hosts = %#v", parsed.Hosts)
	}
	if !parsed.AuthSourceSet || parsed.AuthSource != "admin" {
		t.Fatalf("authSource=%q set=%t", parsed.AuthSource, parsed.AuthSourceSet)
	}
}

func TestMongoGetURIEscapesCredentials(t *testing.T) {
	t.Parallel()

	uri := (Mongo{
		Host:       "localhost",
		Port:       27017,
		Username:   "admin",
		Password:   "p@ss word",
		AuthSource: "admin",
		DBName:     "f_ccg_video",
	}).GetURI()

	parsed, err := connstring.Parse(uri)
	if err != nil {
		t.Fatalf("parse uri: %v", err)
	}
	if parsed.Username != "admin" || parsed.Password != "p@ss word" || parsed.AuthSource != "admin" {
		t.Fatalf("parsed user=%q password=%q authSource=%q", parsed.Username, parsed.Password, parsed.AuthSource)
	}
}

func TestMongoHasNoURIField(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Mongo{})
	if _, ok := typ.FieldByName("URI"); ok {
		t.Fatal("Mongo still has a URI field")
	}

	for i := range typ.NumField() {
		field := typ.Field(i)
		if field.Tag.Get("mapstructure") == "uri" || field.Tag.Get("json") == "uri" {
			t.Fatalf("field %s still maps uri", field.Name)
		}
	}
}
