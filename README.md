# addyui

[addy.io](https://addy.io/) terminal user interface.

# PREVIEW

![addyui demo](vhs/demo.gif)

# DEPENDENCIES

* [Go](https://go.dev/)
* [Make](https://www.gnu.org/software/make/)

# INSTALL

```sh
git clone https://github.com/kovmir/addyui.git
cd addyui
make
make install
```

# USAGE

[Create a new addy.io token](https://app.addy.io/settings/api), and set
`ADDY_API_KEY=your_token`. Invoke `addyui -help` to see the available CLI
flags. Invoke `addyui` to start the program. Hit `F1` for help in the TUI.
