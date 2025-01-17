// Code generated by ent, DO NOT EDIT.

package ent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/hesusruiz/vcbackend/ent/publickey"
)

// PublicKey is the model entity for the PublicKey schema.
type PublicKey struct {
	config `json:"-"`
	// ID of the ent.
	ID string `json:"id,omitempty"`
	// Kty holds the value of the "kty" field.
	Kty string `json:"kty,omitempty"`
	// Alg holds the value of the "alg" field.
	Alg string `json:"alg,omitempty"`
	// Jwk holds the value of the "jwk" field.
	Jwk []uint8 `json:"jwk,omitempty"`
	// CreatedAt holds the value of the "created_at" field.
	CreatedAt time.Time `json:"created_at,omitempty"`
	// UpdatedAt holds the value of the "updated_at" field.
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// scanValues returns the types for scanning values from sql.Rows.
func (*PublicKey) scanValues(columns []string) ([]interface{}, error) {
	values := make([]interface{}, len(columns))
	for i := range columns {
		switch columns[i] {
		case publickey.FieldJwk:
			values[i] = new([]byte)
		case publickey.FieldID, publickey.FieldKty, publickey.FieldAlg:
			values[i] = new(sql.NullString)
		case publickey.FieldCreatedAt, publickey.FieldUpdatedAt:
			values[i] = new(sql.NullTime)
		default:
			return nil, fmt.Errorf("unexpected column %q for type PublicKey", columns[i])
		}
	}
	return values, nil
}

// assignValues assigns the values that were returned from sql.Rows (after scanning)
// to the PublicKey fields.
func (pk *PublicKey) assignValues(columns []string, values []interface{}) error {
	if m, n := len(values), len(columns); m < n {
		return fmt.Errorf("mismatch number of scan values: %d != %d", m, n)
	}
	for i := range columns {
		switch columns[i] {
		case publickey.FieldID:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field id", values[i])
			} else if value.Valid {
				pk.ID = value.String
			}
		case publickey.FieldKty:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field kty", values[i])
			} else if value.Valid {
				pk.Kty = value.String
			}
		case publickey.FieldAlg:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field alg", values[i])
			} else if value.Valid {
				pk.Alg = value.String
			}
		case publickey.FieldJwk:
			if value, ok := values[i].(*[]byte); !ok {
				return fmt.Errorf("unexpected type %T for field jwk", values[i])
			} else if value != nil && len(*value) > 0 {
				if err := json.Unmarshal(*value, &pk.Jwk); err != nil {
					return fmt.Errorf("unmarshal field jwk: %w", err)
				}
			}
		case publickey.FieldCreatedAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field created_at", values[i])
			} else if value.Valid {
				pk.CreatedAt = value.Time
			}
		case publickey.FieldUpdatedAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field updated_at", values[i])
			} else if value.Valid {
				pk.UpdatedAt = value.Time
			}
		}
	}
	return nil
}

// Update returns a builder for updating this PublicKey.
// Note that you need to call PublicKey.Unwrap() before calling this method if this PublicKey
// was returned from a transaction, and the transaction was committed or rolled back.
func (pk *PublicKey) Update() *PublicKeyUpdateOne {
	return (&PublicKeyClient{config: pk.config}).UpdateOne(pk)
}

// Unwrap unwraps the PublicKey entity that was returned from a transaction after it was closed,
// so that all future queries will be executed through the driver which created the transaction.
func (pk *PublicKey) Unwrap() *PublicKey {
	_tx, ok := pk.config.driver.(*txDriver)
	if !ok {
		panic("ent: PublicKey is not a transactional entity")
	}
	pk.config.driver = _tx.drv
	return pk
}

// String implements the fmt.Stringer.
func (pk *PublicKey) String() string {
	var builder strings.Builder
	builder.WriteString("PublicKey(")
	builder.WriteString(fmt.Sprintf("id=%v, ", pk.ID))
	builder.WriteString("kty=")
	builder.WriteString(pk.Kty)
	builder.WriteString(", ")
	builder.WriteString("alg=")
	builder.WriteString(pk.Alg)
	builder.WriteString(", ")
	builder.WriteString("jwk=")
	builder.WriteString(fmt.Sprintf("%v", pk.Jwk))
	builder.WriteString(", ")
	builder.WriteString("created_at=")
	builder.WriteString(pk.CreatedAt.Format(time.ANSIC))
	builder.WriteString(", ")
	builder.WriteString("updated_at=")
	builder.WriteString(pk.UpdatedAt.Format(time.ANSIC))
	builder.WriteByte(')')
	return builder.String()
}

// PublicKeys is a parsable slice of PublicKey.
type PublicKeys []*PublicKey

func (pk PublicKeys) config(cfg config) {
	for _i := range pk {
		pk[_i].config = cfg
	}
}
