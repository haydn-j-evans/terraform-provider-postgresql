package postgresql

import (
	"testing"

	"github.com/blang/semver"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"
)

func TestFindStringSubmatchMap(t *testing.T) {

	resultMap := findStringSubmatchMap(`(?si).*\$(?P<Body>.*)\$.*`, "aa $something_to_extract$ bb")

	assert.Equal(t,
		resultMap,
		map[string]string{
			"Body": "something_to_extract",
		},
	)
}

func TestQuoteTableName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple table name",
			input:    "users",
			expected: `"users"`,
		},
		{
			name:     "table name with schema",
			input:    "test.users",
			expected: `"test"."users"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := quoteTableName(tt.input)
			if actual != tt.expected {
				t.Errorf("quoteTableName() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestArePrivilegesEqual(t *testing.T) {

	pg16 := semver.MustParse("16.0.0")
	pg17 := semver.MustParse("17.0.0")

	type PrivilegesTestObject struct {
		name      string
		d         *schema.ResourceData
		granted   *schema.Set
		wanted    *schema.Set
		version   semver.Version
		assertion bool
	}

	tt := []PrivilegesTestObject{
		{
			"database ALL on PG17",
			buildResourceData("database", t),
			buildPrivilegesSet("CONNECT", "CREATE", "TEMPORARY"),
			buildPrivilegesSet("ALL"),
			pg17,
			true,
		},
		{
			"database non-matching privileges",
			buildResourceData("database", t),
			buildPrivilegesSet("CREATE", "USAGE"),
			buildPrivilegesSet("USAGE"),
			pg17,
			false,
		},
		{
			"table ALL on PG17 (includes MAINTAIN)",
			buildResourceData("table", t),
			buildPrivilegesSet("SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN"),
			buildPrivilegesSet("ALL"),
			pg17,
			true,
		},
		{
			"table ALL on PG16 (no MAINTAIN)",
			buildResourceData("table", t),
			buildPrivilegesSet("SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"),
			buildPrivilegesSet("ALL"),
			pg16,
			true,
		},
		{
			"table ALL on PG16 with MAINTAIN should fail",
			buildResourceData("table", t),
			buildPrivilegesSet("SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN"),
			buildPrivilegesSet("ALL"),
			pg16,
			false,
		},
		{
			"table ALL on PG17 without MAINTAIN should fail",
			buildResourceData("table", t),
			buildPrivilegesSet("SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"),
			buildPrivilegesSet("ALL"),
			pg17,
			false,
		},
		{
			"table non-matching privileges",
			buildResourceData("table", t),
			buildPrivilegesSet("SELECT"),
			buildPrivilegesSet("SELECT, INSERT"),
			pg17,
			false,
		},
		{
			"schema ALL on PG17",
			buildResourceData("schema", t),
			buildPrivilegesSet("CREATE", "USAGE"),
			buildPrivilegesSet("ALL"),
			pg17,
			true,
		},
		{
			"schema partial privileges should not match ALL",
			buildResourceData("schema", t),
			buildPrivilegesSet("CREATE"),
			buildPrivilegesSet("ALL"),
			pg17,
			false,
		},
	}

	for _, configuration := range tt {
		t.Run(configuration.name, func(t *testing.T) {
			err := configuration.d.Set("privileges", configuration.wanted)
			assert.NoError(t, err)
			equal := resourcePrivilegesEqual(configuration.granted, configuration.d, configuration.version)
			assert.Equal(t, configuration.assertion, equal)
		})
	}
}

func buildPrivilegesSet(grants ...any) *schema.Set {
	return schema.NewSet(schema.HashString, grants)
}

func buildResourceData(objectType string, t *testing.T) *schema.ResourceData {
	var testSchema = map[string]*schema.Schema{
		"object_type": {Type: schema.TypeString},
		"privileges": {
			Type: schema.TypeSet,
			Elem: &schema.Schema{Type: schema.TypeString},
			Set:  schema.HashString,
		},
	}

	m := make(map[string]any)
	m["object_type"] = objectType
	return schema.TestResourceDataRaw(t, testSchema, m)
}
