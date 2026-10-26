package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
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

	{
		err := v.ReadConfig(strings.NewReader(raw))
		require.NoError(t, err, "read config: %v", err)
	}

	var cfg Config
	{
		err := v.Unmarshal(&cfg)
		require.NoError(t, err, "unmarshal: %v", err)
	}

	require.NotNil(t, cfg.Mongo, "mongo config is nil")

	require.Equal(t, "192.168.1.3", cfg.Mongo.Host, "mongo address = %+v", cfg.Mongo)
	require.Equal(t, 27017, cfg.Mongo.Port, "mongo address = %+v", cfg.Mongo)
	require.Equal(t, "f_ccg_video", cfg.Mongo.DBName, "mongo address = %+v", cfg.Mongo)

	require.Equal(t, testAdmin, cfg.Mongo.Username, "mongo auth = %+v", cfg.Mongo)
	require.Equal(t, "123456", cfg.Mongo.Password, "mongo auth = %+v", cfg.Mongo)
	require.Equal(t, testAdmin, cfg.Mongo.AuthSource, "mongo auth = %+v", cfg.Mongo)

	require.Equal(t, 5, cfg.Mongo.TimeoutSec, "mongo pool = %+v", cfg.Mongo)
	require.EqualValues(t, 50, cfg.Mongo.MaxPool, "mongo pool = %+v", cfg.Mongo)

	uri := cfg.Mongo.GetURI()
	require.NotContains(t, uri, "stale.example", "GetURI = %q", uri)
	require.NotContains(t, uri, "otherdb", "GetURI = %q", uri)

	parsed, err := connstring.Parse(uri)
	require.NoError(t, err, "parse uri: %v", err)

	{
		err = options.Client().ApplyURI(uri).Validate()
		require.NoError(t, err, "ApplyURI: %v", err)
	}

	require.Equal(t, testAdmin, parsed.Username, "user=%q password=%q", parsed.Username, parsed.Password)
	require.Equal(t, "123456", parsed.Password, "user=%q password=%q", parsed.Username, parsed.Password)

	require.Len(t, parsed.Hosts, 1, "hosts = %#v", parsed.Hosts)
	require.Equal(t, "192.168.1.3:27017", parsed.Hosts[0], "hosts = %#v", parsed.Hosts)

	require.True(t, parsed.AuthSourceSet, "authSource=%q set=%t", parsed.AuthSource, parsed.AuthSourceSet)
	require.Equal(t, testAdmin, parsed.AuthSource, "authSource=%q set=%t", parsed.AuthSource, parsed.AuthSourceSet)
}

func TestMongoGetURIEscapesCredentials(t *testing.T) {
	t.Parallel()

	uri := (Mongo{
		Host:       "localhost",
		Port:       27017,
		Username:   testAdmin,
		Password:   "p@ss word",
		AuthSource: testAdmin,
		DBName:     "f_ccg_video",
	}).GetURI()

	parsed, err := connstring.Parse(uri)
	require.NoError(t, err, "parse uri: %v", err)

	require.Equal(t, testAdmin, parsed.Username, "parsed user=%q password=%q authSource=%q", parsed.Username, parsed.Password, parsed.AuthSource)
	require.Equal(t, "p@ss word", parsed.Password, "parsed user=%q password=%q authSource=%q", parsed.Username, parsed.Password, parsed.AuthSource)
	require.Equal(t, testAdmin, parsed.AuthSource, "parsed user=%q password=%q authSource=%q", parsed.Username, parsed.Password, parsed.AuthSource)
}

func TestMongoHasNoURIField(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeFor[Mongo]()
	{
		_, ok := typ.FieldByName("URI")
		require.False(t, ok, "Mongo still has a URI field")
	}

	for i := range typ.NumField() {
		field := typ.Field(i)
		require.NotEqual(t, "uri", field.Tag.Get("mapstructure"), "field %s still maps uri", field.Name)
		require.NotEqual(t, "uri", field.Tag.Get("json"), "field %s still maps uri", field.Name)
	}
}
