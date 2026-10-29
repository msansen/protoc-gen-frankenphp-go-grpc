# TODO

- [ ] Fix map decoding.
  - Use the `key` variable in the template instead of the field name.
  - Select the value converter based on the map's value field.
  - Correctly convert numeric and boolean keys.
  - Verify compilation and map decoding in RPC responses.

- [ ] Support multiple `.proto` files in the same Go package.
  - Avoid duplicate declarations of helpers and shared message codecs.
  - Verify compilation of multiple generated files containing services in the same package.

- [ ] Fix imported type references and affected scalar encoding.
  - Qualify types from other packages, including repeated message types, enums, and oneof wrappers.
  - Dereference optional enums before conversion.
  - Register and qualify the `strconv` import required to encode `uint64` and `fixed64` values.
  - Add compilation cases covering imported types, optional enums, and unsigned 64-bit integers.
