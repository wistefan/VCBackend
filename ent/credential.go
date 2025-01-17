// Code generated by ent, DO NOT EDIT.

package ent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/hesusruiz/vcbackend/ent/credential"
	"github.com/hesusruiz/vcbackend/ent/user"
)

// Credential is the model entity for the Credential schema.
type Credential struct {
	config `json:"-"`
	// ID of the ent.
	ID string `json:"id,omitempty"`
	// Type holds the value of the "type" field.
	Type string `json:"type,omitempty"`
	// Raw holds the value of the "raw" field.
	Raw []uint8 `json:"raw,omitempty"`
	// CreatedAt holds the value of the "created_at" field.
	CreatedAt time.Time `json:"created_at,omitempty"`
	// UpdatedAt holds the value of the "updated_at" field.
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	// Edges holds the relations/edges for other nodes in the graph.
	// The values are being populated by the CredentialQuery when eager-loading is set.
	Edges                      CredentialEdges `json:"edges"`
	natural_person_credentials *string
	user_credentials           *string
}

// CredentialEdges holds the relations/edges for other nodes in the graph.
type CredentialEdges struct {
	// Account holds the value of the account edge.
	Account *User `json:"account,omitempty"`
	// loadedTypes holds the information for reporting if a
	// type was loaded (or requested) in eager-loading or not.
	loadedTypes [1]bool
}

// AccountOrErr returns the Account value or an error if the edge
// was not loaded in eager-loading, or loaded but was not found.
func (e CredentialEdges) AccountOrErr() (*User, error) {
	if e.loadedTypes[0] {
		if e.Account == nil {
			// The edge account was loaded in eager-loading,
			// but was not found.
			return nil, &NotFoundError{label: user.Label}
		}
		return e.Account, nil
	}
	return nil, &NotLoadedError{edge: "account"}
}

// scanValues returns the types for scanning values from sql.Rows.
func (*Credential) scanValues(columns []string) ([]interface{}, error) {
	values := make([]interface{}, len(columns))
	for i := range columns {
		switch columns[i] {
		case credential.FieldRaw:
			values[i] = new([]byte)
		case credential.FieldID, credential.FieldType:
			values[i] = new(sql.NullString)
		case credential.FieldCreatedAt, credential.FieldUpdatedAt:
			values[i] = new(sql.NullTime)
		case credential.ForeignKeys[0]: // natural_person_credentials
			values[i] = new(sql.NullString)
		case credential.ForeignKeys[1]: // user_credentials
			values[i] = new(sql.NullString)
		default:
			return nil, fmt.Errorf("unexpected column %q for type Credential", columns[i])
		}
	}
	return values, nil
}

// assignValues assigns the values that were returned from sql.Rows (after scanning)
// to the Credential fields.
func (c *Credential) assignValues(columns []string, values []interface{}) error {
	if m, n := len(values), len(columns); m < n {
		return fmt.Errorf("mismatch number of scan values: %d != %d", m, n)
	}
	for i := range columns {
		switch columns[i] {
		case credential.FieldID:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field id", values[i])
			} else if value.Valid {
				c.ID = value.String
			}
		case credential.FieldType:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field type", values[i])
			} else if value.Valid {
				c.Type = value.String
			}
		case credential.FieldRaw:
			if value, ok := values[i].(*[]byte); !ok {
				return fmt.Errorf("unexpected type %T for field raw", values[i])
			} else if value != nil && len(*value) > 0 {
				if err := json.Unmarshal(*value, &c.Raw); err != nil {
					return fmt.Errorf("unmarshal field raw: %w", err)
				}
			}
		case credential.FieldCreatedAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field created_at", values[i])
			} else if value.Valid {
				c.CreatedAt = value.Time
			}
		case credential.FieldUpdatedAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field updated_at", values[i])
			} else if value.Valid {
				c.UpdatedAt = value.Time
			}
		case credential.ForeignKeys[0]:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field natural_person_credentials", values[i])
			} else if value.Valid {
				c.natural_person_credentials = new(string)
				*c.natural_person_credentials = value.String
			}
		case credential.ForeignKeys[1]:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field user_credentials", values[i])
			} else if value.Valid {
				c.user_credentials = new(string)
				*c.user_credentials = value.String
			}
		}
	}
	return nil
}

// QueryAccount queries the "account" edge of the Credential entity.
func (c *Credential) QueryAccount() *UserQuery {
	return (&CredentialClient{config: c.config}).QueryAccount(c)
}

// Update returns a builder for updating this Credential.
// Note that you need to call Credential.Unwrap() before calling this method if this Credential
// was returned from a transaction, and the transaction was committed or rolled back.
func (c *Credential) Update() *CredentialUpdateOne {
	return (&CredentialClient{config: c.config}).UpdateOne(c)
}

// Unwrap unwraps the Credential entity that was returned from a transaction after it was closed,
// so that all future queries will be executed through the driver which created the transaction.
func (c *Credential) Unwrap() *Credential {
	_tx, ok := c.config.driver.(*txDriver)
	if !ok {
		panic("ent: Credential is not a transactional entity")
	}
	c.config.driver = _tx.drv
	return c
}

// String implements the fmt.Stringer.
func (c *Credential) String() string {
	var builder strings.Builder
	builder.WriteString("Credential(")
	builder.WriteString(fmt.Sprintf("id=%v, ", c.ID))
	builder.WriteString("type=")
	builder.WriteString(c.Type)
	builder.WriteString(", ")
	builder.WriteString("raw=")
	builder.WriteString(fmt.Sprintf("%v", c.Raw))
	builder.WriteString(", ")
	builder.WriteString("created_at=")
	builder.WriteString(c.CreatedAt.Format(time.ANSIC))
	builder.WriteString(", ")
	builder.WriteString("updated_at=")
	builder.WriteString(c.UpdatedAt.Format(time.ANSIC))
	builder.WriteByte(')')
	return builder.String()
}

// Credentials is a parsable slice of Credential.
type Credentials []*Credential

func (c Credentials) config(cfg config) {
	for _i := range c {
		c[_i].config = cfg
	}
}
