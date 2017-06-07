package redfishserver

import (
	. "github.com/smartystreets/goconvey/convey"
	"testing"

	"encoding/json"
)

func TestSimpleJQL(t *testing.T) {
	// Only pass t into top-level Convey calls
	Convey("Given a JSON String with key1 and key2", t, func() {
		str := `{ "key1": "value1", "key2": "value2", "array": [1,2,3,4] }`

		Convey("that we unmarshal into an interface{}", func() {
			var data interface{}
			err := json.Unmarshal([]byte(str), &data)
			Convey("We should not get any error from the unmarshal", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("we should get 'value1' back when we query .key1", func() {
				result, err := SimpleJQL(data, ".key1")
				Convey("and we should not get any error", func() {
					So(err, ShouldEqual, nil)
				})
				So(result.(string), ShouldEqual, "value1")
			})

			Convey("we should get 'value1' back when we query key1", func() {
				result, err := SimpleJQL(data, ".key1")
				Convey("and we should not get any error", func() {
					So(err, ShouldEqual, nil)
				})
				So(result.(string), ShouldEqual, "value1")
			})

			Convey("We should get 'value2' back when we query .key2", func() {
				result, err := SimpleJQL(data, ".key2")
				Convey("and we should not get any error", func() {
					So(err, ShouldEqual, nil)
				})
				So(result.(string), ShouldEqual, "value2")
			})
			Convey("We should get an error if we query a .nonexistent key", func() {
				result, err := SimpleJQL(data, ".nonexistent")
				Convey("we should get an error return", func() {
					So(err.Error(), ShouldEqual, "nonexistent no such element")
				})
				So(result, ShouldEqual, nil)
			})

			Convey("We should get an error if we query by array", func() {
				result, err := SimpleJQL(data, ".[4]")
				Convey("we should get an error return", func() {
					So(err.Error(), ShouldEqual, "attempt to array index into a non-array")
				})
				So(result, ShouldEqual, nil)
			})

			Convey("We should get an error if we query array by non-numeric", func() {
				result, err := SimpleJQL(data, ".[a]")
				Convey("we should get an error return", func() {
					So(err.Error(), ShouldEqual, `strconv.Atoi: parsing "a": invalid syntax`)
				})
				So(result, ShouldEqual, nil)
			})

			Convey("if we query first array element, it should equal 1", func() {
				result, err := SimpleJQL(data, ".array.[0]")
				Convey("we should get an error return", func() {
					So(err, ShouldEqual, nil)
				})
				So(result.(float64), ShouldEqual, 1)
			})
		})
	})
}
