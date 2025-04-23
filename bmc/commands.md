

```
# tpi help
Official Turing-Pi2 CLI tool

Usage: tpi [OPTIONS] [COMMAND]

Commands:
  power     Power on/off or reset specific nodes
  usb       Change the USB device/host configuration. The USB-bus can only be routed to one node simultaneously
  firmware  Upgrade the firmware of the BMC
  flash     Flash a given node
  eth       Configure the on-board Ethernet switch
  uart      Read or write over UART
  advanced  Advanced node modes
  info      Print turing-pi info
  reboot    Reboot the BMC chip. Nodes will lose power until booted!
  help      Print this message or the help of the given subcommand(s)

Options:
      --host <HOST>        Specify the Turing-pi host to connect to. Note: IPv6 addresses must be wrapped in square brackets e.g. `[::1]` [default: 127.0.0.1]
      --port <PORT>        Specify a custom port to connect to
      --user <USER>        Specify a user name to log in as. If unused, an interactive prompt will ask for credentials unless a cached token file is present
      --password <PASS>    Same as `--username`
      --json               Print results formatted as JSON
  -a <API_VERSION>         Force which version of the BMC API to use. Try lower the version if you are running older BMC firmware [default: v1-1] [possible values: v1, v1-1]
  -g <gen completion>      [possible values: bash, elvish, fish, powershell, zsh]
  -h, --help               Print help
  -V, --version            Print version
# tpi power
Power on/off or reset specific nodes

Usage: tpi power [OPTIONS] <CMD>

Arguments:
  <CMD>  Specify command [possible values: on, off, reset, status]

Options:
  -n, --node <NODE>      [possible values: 1-4], Not specifying a node selects all nodes
      --host <HOST>      Specify the Turing-pi host to connect to. Note: IPv6 addresses must be wrapped in square brackets e.g. `[::1]` [default: 127.0.0.1]
      --port <PORT>      Specify a custom port to connect to
      --user <USER>      Specify a user name to log in as. If unused, an interactive prompt will ask for credentials unless a cached token file is present
      --password <PASS>  Same as `--username`
      --json             Print results formatted as JSON
  -a <API_VERSION>       Force which version of the BMC API to use. Try lower the version if you are running older BMC firmware [default: v1-1] [possible values: v1, v1-1]
  -h, --help             Print help
  -V, --version          Print version
# tpi uart
Read or write over UART

Usage: tpi uart [OPTIONS] --node <NODE> <ACTION>

Arguments:
  <ACTION>  [possible values: get, set]

Options:
  -n, --node <NODE>      [possible values: 1-4], Not specifying a node selects all nodes
  -c, --cmd <CMD>        
      --host <HOST>      Specify the Turing-pi host to connect to. Note: IPv6 addresses must be wrapped in square brackets e.g. `[::1]` [default: 127.0.0.1]
      --port <PORT>      Specify a custom port to connect to
      --user <USER>      Specify a user name to log in as. If unused, an interactive prompt will ask for credentials unless a cached token file is present
      --password <PASS>  Same as `--username`
      --json             Print results formatted as JSON
  -a <API_VERSION>       Force which version of the BMC API to use. Try lower the version if you are running older BMC firmware [default: v1-1] [possible values: v1, v1-1]
  -h, --help             Print help
  -V, --version          Print version
# tpi info
|---key----|-----------value------------|
 api       : 1.1
 build_version: 2023.08
 buildroot : "Buildroot 2023.08"
 buildtime : 2023-11-28 14:01:07-00:00
 ip        : 192.168.1.90
 mac       : 02:00:4a:ea:dd:34

 version   : 2.0.5
|----------|----------------------------|
# tpi eth
Configure the on-board Ethernet switch

Usage: tpi eth [OPTIONS] <CMD>

Arguments:
  <CMD>  Specify command [possible values: reset]

Options:
      --host <HOST>      Specify the Turing-pi host to connect to. Note: IPv6 addresses must be wrapped in square brackets e.g. `[::1]` [default: 127.0.0.1]
      --port <PORT>      Specify a custom port to connect to
      --user <USER>      Specify a user name to log in as. If unused, an interactive prompt will ask for credentials unless a cached token file is present
      --password <PASS>  Same as `--username`
      --json             Print results formatted as JSON
  -a <API_VERSION>       Force which version of the BMC API to use. Try lower the version if you are running older BMC firmware [default: v1-1] [possible values: v1, v1-1]
  -h, --help             Print help
  -V, --version          Print version
# tpi advanced
Advanced node modes

Usage: tpi advanced [OPTIONS] --node <NODE> <MODE>

Arguments:
  <MODE>  [possible values: normal, msd]

Options:
  -n, --node <NODE>      [possible values: 1-4]
      --host <HOST>      Specify the Turing-pi host to connect to. Note: IPv6 addresses must be wrapped in square brackets e.g. `[::1]` [default: 127.0.0.1]
      --port <PORT>      Specify a custom port to connect to
      --user <USER>      Specify a user name to log in as. If unused, an interactive prompt will ask for credentials unless a cached token file is present
      --password <PASS>  Same as `--username`
      --json             Print results formatted as JSON
  -a <API_VERSION>       Force which version of the BMC API to use. Try lower the version if you are running older BMC firmware [default: v1-1] [possible values: v1, v1-1]
  -h, --help             Print help (see more with '--help')
  -V, --version          Print version
# tpi uart
Read or write over UART

Usage: tpi uart [OPTIONS] --node <NODE> <ACTION>

Arguments:
  <ACTION>  [possible values: get, set]

Options:
  -n, --node <NODE>      [possible values: 1-4], Not specifying a node selects all nodes
  -c, --cmd <CMD>        
      --host <HOST>      Specify the Turing-pi host to connect to. Note: IPv6 addresses must be wrapped in square brackets e.g. `[::1]` [default: 127.0.0.1]
      --port <PORT>      Specify a custom port to connect to
      --user <USER>      Specify a user name to log in as. If unused, an interactive prompt will ask for credentials unless a cached token file is present
      --password <PASS>  Same as `--username`
      --json             Print results formatted as JSON
  -a <API_VERSION>       Force which version of the BMC API to use. Try lower the version if you are running older BMC firmware [default: v1-1] [possible values: v1, v1-1]
  -h, --help             Print help
  -V, --version          Print version
# 
```