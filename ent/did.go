// Code generated by ent, DO NOT EDIT.

package ent

import (
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/hesusruiz/vcbackend/ent/did"
	"github.com/hesusruiz/vcbackend/ent/user"
)

// DID is the model entity for the DID schema.
type DID struct {
	config `json:"-"`
	// ID of the ent.
	ID string `json:"id,omitempty"`
	// Method holds the value of the "method" field.
	Method string `json:"method,omitempty"`
	// CreatedAt holds the value of the "created_at" field.
	CreatedAt time.Time `json:"created_at,omitempty"`
	// UpdatedAt holds the value of the "updated_at" field.
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	// Edges holds the relations/edges for other nodes in the graph.
	// The values are being populated by the DIDQuery when eager-loading is set.
	Edges     DIDEdges `json:"edges"`
	user_dids *string
}

// DIDEdges holds the relations/edges for other nodes in the graph.
type DIDEdges struct {
	// User holds the value of the user edge.
	User *User `json:"user,omitempty"`
	// loadedTypes holds the information for reporting if a
	// type was loaded (or requested) in eager-loading or not.
	loadedTypes [1]bool
}

// UserOrErr returns the User value or an error if the edge
// was not loaded in eager-loading, or loaded but was not found.
func (e DIDEdges) UserOrErr() (*User, error) {
	if e.loadedTypes[0] {
		if e.User == nil {
			// The edge user was loaded in eager-loading,
			// but was not found.
			return nil, &NotFoundError{label: user.Label}
		}
		return e.User, nil
	}
	return nil, &NotLoadedError{edge: "user"}
}

// scanValues returns the types for scanning values from sql.Rows.
func (*DID) scanValues(columns []string) ([]interface{}, error) {
	values := make([]interface{}, len(columns))
	for i := range columns {
		switch columns[i] {
		case did.FieldID, did.FieldMethod:
			values[i] = new(sql.NullString)
		case did.FieldCreatedAt, did.FieldUpdatedAt:
			values[i] = new(sql.NullTime)
		case did.ForeignKeys[0]: // user_dids
			values[i] = new(sql.NullString)
		default:
			return nil, fmt.Errorf("unexpected column %q for type DID", columns[i])
		}
	}
	return values, nil
}

// assignValues assigns the values that were returned from sql.Rows (after scanning)
// to the DID fields.
func (d *DID) assignValues(columns []string, values []interface{}) error {
	if m, n := len(values), len(columns); m < n {
		return fmt.Errorf("mismatch number of scan values: %d != %d", m, n)
	}
	for i := range columns {
		switch columns[i] {
		case did.FieldID:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field id", values[i])
			} else if value.Valid {
				d.ID = value.String
			}
		case did.FieldMethod:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field method", values[i])
			} else if value.Valid {
				d.Method = value.String
			}
		case did.FieldCreatedAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field created_at", values[i])
			} else if value.Valid {
				d.CreatedAt = value.Time
			}
		case did.FieldUpdatedAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field updated_at", values[i])
			} else if value.Valid {
				d.UpdatedAt = value.Time
			}
		case did.ForeignKeys[0]:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field user_dids", values[i])
			} else if value.Valid {
				d.user_dids = new(string)
				*d.user_dids = value.String
			}
		}
	}
	return nil
}

// QueryUser queries the "user" edge of the DID entity.
func (d *DID) QueryUser() *UserQuery {
	return (&DIDClient{config: d.config}).QueryUser(d)
}

// Update returns a builder for updating this DID.
// Note that you need to call DID.Unwrap() before calling this method if this DID
// was returned from a transaction, and the transaction was committed or rolled back.
func (d *DID) Update() *DIDUpdateOne {
	return (&DIDClient{config: d.config}).UpdateOne(d)
}

// Unwrap unwraps the DID entity that was returned from a transaction after it was closed,
// so that all future queries will be executed through the driver which created the transaction.
func (d *DID) Unwrap() *DID {
	_tx, ok := d.config.driver.(*txDriver)
	if !ok {
		panic("ent: DID is not a transactional entity")
	}
	d.config.driver = _tx.drv
	return d
}

// String implements the fmt.Stringer.
func (d *DID) String() string {
	var builder strings.Builder
	builder.WriteString("DID(")
	builder.WriteString(fmt.Sprintf("id=%v, ", d.ID))
	builder.WriteString("method=")
	builder.WriteString(d.Method)
	builder.WriteString(", ")
	builder.WriteString("created_at=")
	builder.WriteString(d.CreatedAt.Format(time.ANSIC))
	builder.WriteString(", ")
	builder.WriteString("updated_at=")
	builder.WriteString(d.UpdatedAt.Format(time.ANSIC))
	builder.WriteByte(')')
	return builder.String()
}

// DIDs is a parsable slice of DID.
type DIDs []*DID

func (d DIDs) config(cfg config) {
	for _i := range d {
		d[_i].config = cfg
	}
}
