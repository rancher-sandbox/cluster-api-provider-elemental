# go-vfsafero

[![PkgGoDev](https://pkg.go.dev/badge/github.com/twpayne/go-vfsafero/v4)](https://pkg.go.dev/github.com/twpayne/go-vfsafero/v4)

Package `vfsafero` provides a compatibility later between
[`github.com/twpayne/go-vfs`](https://github.com/twpayne/go-vfs) and
[`github.com/spf13/afero`](https://github.com/spf13/afero).

This allows you to use `vfst` to test exisiting code that uses
[`afero.Fs`](https://pkg.go.dev/github.com/spf13/afero#Fs), and use `vfs.FS`s as
`afero.Fs`s. See [the
documentation](https://pkg.go.dev/github.com/twpayne/go-vfsafero/v4) for an example.

## License

MIT
