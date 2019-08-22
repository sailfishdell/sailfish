package godefs

import "reflect"

func NewStruct(dest interface{}, r io.ByteReader) error {

	value := reflect.ValueOf(dest).Elem()
	typ := value.Type()

	for i := 0; i < value.NumField(); i++ {
		sfl := value.Field(i)
		fl := typ.Field(i)
		rawSeek := fl.Tag.Get("seek")
		if len(rawSeek) > 0 {
			base := 10
			if strings.HasPrefix(rawSeek, "0x") {
				base = 16
				rawSeek = rawSeek[2:]
			}
			seek, err := strconv.ParseUint(rawSeek, base, 32)
			if err != nil {
				return err
			}

			seeker, ok := r.(io.Seeker)
			if !ok {
				return errors.New("io.Seeker interface is required")
			}
			seeker.Seek(int64(seek), 0)
		}

		if sfl.IsValid() && sfl.CanSet() {
			var err error
			switch fl.Type.Kind() {
			case reflect.Uint8:
				u8, err := readU8(r)
				if err == nil {
					sfl.SetUint(uint64(u8))
				}
			case reflect.Uint16:
				u16, err := readU16(r)
				if err == nil {
					sfl.SetUint(uint64(u16))
				}
			case reflect.Uint32:
				u32, err := readU32(r)
				if err == nil {
					sfl.SetUint(uint64(u32))
				}
			case reflect.Uint64:
				u64, err := readU64(r)
				if err == nil {
					sfl.SetUint(u64)
				}

			case reflect.Slice:
				if sfl.Type() == reflect.TypeOf(uuid.UUID{}) {
					uuidValue, err := readUUID(r.(io.Reader))
					if err == nil {
						sfl.SetBytes([]byte(uuidValue))
					}
				} else {

				}

			case reflect.Struct:
				err = NewStruct(sfl.Addr().Interface(), r)
			}

			if err != nil {
				return err
			}
		}
	}

	return nil
}
