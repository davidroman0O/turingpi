# Turing Pi 2 CLI – BMC and Compute Node Management

## Overview of Turing Pi 2 CLI (BMC Management)

The Turing Pi 2 comes with an official command-line tool called **`tpi`**, used for managing the Baseboard Management Controller (BMC) and the compute nodes. This CLI is pre-installed on the BMC (accessible via SSH or serial) and can also be installed on your PC (Windows, macOS, or Linux) for remote management ([Overview](https://docs.turingpi.com/docs/tpi-overview#:~:text=The%20tpi%20is%20a%20tool,installed%20on%20your%20Windows%2FmacOS%2FLinux%20PC)) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=Options%3A%20,cached%20token%20file%20is%20present)). Using `tpi`, you can power nodes on/off, reset them, configure USB and network settings, flash OS images to nodes, upgrade BMC firmware, access serial consoles, and more. The CLI communicates with the BMC’s API over the network, connecting by default to the hostname **`turingpi.local`** (you can specify a different host or IP if needed) ([Overview](https://docs.turingpi.com/docs/tpi-overview#:~:text=Or%20directly%20from%20your%20personal,host%60%20option)).

**Prerequisites & Setup:**

- **BMC Network Access:** Ensure the BMC is powered on and connected to your network. If using the CLI from a PC, you may reach it at `turingpi.local` by default, or use `--host <BMC_IP_or_hostname>` to specify the address ([Overview](https://docs.turingpi.com/docs/tpi-overview#:~:text=Or%20directly%20from%20your%20personal,host%60%20option)). If mDNS is blocked on your network, find the BMC’s IP from your router’s DHCP leases (look for hostname `turingpi`) ([BMC User Interface](https://docs.turingpi.com/docs/turing-pi2-bmc-ui#:~:text=The%20BMC%20User%20Interface%20,is%20the%20IP%20address)).
- **CLI Installation (if remote):** The `tpi` tool can be downloaded as a binary (for Windows, Mac, Linux) or installed via package managers (APT, AUR, Cargo, etc.) ([GitHub - turing-machines/tpi: CLI tool to control your Turing Pi 2 board](https://github.com/turing-machines/tpi#:~:text=,Choose%20one%20of%20the%20following)) ([GitHub - turing-machines/tpi: CLI tool to control your Turing Pi 2 board](https://github.com/turing-machines/tpi#:~:text=You%20can%20now%20install%20the,tpi%20package%20with%20command)). On the BMC itself, `tpi` is already available.
- **Authentication:** For BMC firmware v2.0.0 and above, commands require authentication ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=Starting%20with%20version%202,enter%20username%20and%20password%20interactively)). The default credentials are **user:** `root`, **password:** `turing` ([BMC User Interface](https://docs.turingpi.com/docs/turing-pi2-bmc-ui#:~:text=You%20get%20asked%20to%20authenticate,default%20login%20and%20passwords%20are)) (be sure to change the default password via the BMC UI for security). If you run `tpi` without providing credentials, it will interactively prompt for username and password on first use ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=%24%20tpi%20power%20status%20,is%20entered%20at%20a%20prompt)). You can also supply `--user <USER>` and `--password <PASS>` flags to avoid prompts ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=,1)). After a successful login, a token is cached (e.g. in `~/.cache` on Linux) so subsequent commands (for ~3 hours by default) won’t require re-authentication ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=When%20you%20provide%20both%20credentials,value)) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=The%20file%20with%20an%20authentication,is%20stored%20in%20these%20directories)).
- **System Requirements:** If running on a PC, ensure you have network connectivity to the BMC. No special privileges are required for most `tpi` commands (they operate through the BMC’s API), but installing the CLI may require admin rights. On the BMC, you’ll typically use the `root` account for full access.

## Using the CLI Tool – Syntax and Global Options

The general syntax is: 

```bash
tpi [OPTIONS] <COMMAND> [command-specific arguments/options]
``` 

For example: `tpi power on --node 1` would execute the **power** command to turn on node 1. You can always run `tpi --help` to see the global help, or `tpi <command> --help` for help on a specific subcommand.

**Global Options (apply to any command):**  

- `--host <HOST>` – Specify the BMC host address to connect to (default is `turingpi.local`). This can be an IP or hostname (wrap IPv6 addresses in `[]`) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=Options%3A%20,a%20cached%20token%20file%20is)).  
- `--port <PORT>` – Custom port if the BMC API is running on a non-default port ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=Options%3A%20,a%20cached%20token%20file%20is)). (By default, it uses port 22 for SSH or the appropriate API port for HTTP as configured; usually you won’t need this.)  
- `--user <USER>` – BMC username for authentication (default prompts if not given) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=,1)).  
- `--password <PASS>` – BMC password (you can provide it here or omit to be prompted) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=,1)).  
- `--json` – Output results in JSON format (useful for scripting or feeding into other tools) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=,values%3A%20bash%2C%20elvish%2C%20fish%2C%20powershell)).  
- `-a <API_VERSION>` – Force a specific BMC API version if needed for older firmware compatibility (e.g. `v1` or `v1-1`; defaults to latest) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=,values%3A%20bash%2C%20elvish%2C%20fish%2C%20powershell)).  
- `-g <SHELL>` – Generate shell completion script for the specified shell (`bash`, `zsh`, etc.) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=%5Bpossible%20values%3A%20v1%2C%20v1,Print%20help)).  
- `-h, --help` – Show help information.  
- `-V, --version` – Show the version of the `tpi` tool.

**Authentication Example:** If you don’t include `--user`/`--password`, you’ll be prompted. For instance: 

```bash
$ tpi power status         # will prompt for credentials
User: root
Password: ****
node1: off
node2: off
...
``` 

You can avoid prompts by specifying credentials: 

```bash
$ tpi power status --user root --password turing ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=Password%3A%20node1%3A%20off%20,requested%20interactively%20node1%3A%20off))
node1: off
node2: off
...
``` 

After the first login, an auth token is cached, so subsequent commands won’t ask for password until the token expires ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=When%20you%20provide%20both%20credentials,value)).

**Tip:** If you have multiple Turing Pi 2 boards, ensure you use `--host` with the correct hostname/IP for each. By default, if multiple boards broadcast the same name, they may enumerate as `turingpi-2.local`, `turingpi-3.local`, etc. ([BMC User Interface](https://docs.turingpi.com/docs/turing-pi2-bmc-ui#:~:text=,2)).

## Command List Overview

The `tpi` CLI provides several subcommands for specific management functions. Below is the full list of commands and their primary purpose ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=Commands%3A%20power%20%20%20,Read%20or%20write%20over%20UART)):

- **`power`** – Power on/off or reset nodes (manage power state of compute modules) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=Commands%3A%20power%20%20%20,Read%20or%20write%20over%20UART)).  
- **`usb`** – Configure the USB routing (switch the shared USB OTG bus between nodes or BMC) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=power%20%20%20%20,Read%20or%20write%20over%20UART)).  
- **`firmware`** – Upgrade the BMC’s firmware (update the BMC’s software) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=simultaneously%20firmware%20%20Upgrade%20the,Read%20or%20write%20over%20UART)).  
- **`flash`** – Flash an OS image to a compute node’s storage (e.g. eMMC) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=simultaneously%20firmware%20%20Upgrade%20the,Read%20or%20write%20over%20UART)).  
- **`eth`** – Configure the on-board Ethernet switch and networking settings ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=firmware%20%20Upgrade%20the%20firmware,Read%20or%20write%20over%20UART)).  
- **`uart`** – Read from or write to a node’s serial console (UART) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=flash%20%20%20%20,advanced%20%20Advanced%20node%20modes)).  
- **`advanced`** – Access advanced node modes (e.g. special boot modes like mass storage device mode) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=eth%20%20%20%20,pi%20info)).  
- **`info`** – Show information about the Turing Pi board (IP addresses, MACs, storage, etc.) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=uart%20%20%20%20,pi%20info)).  
- **`reboot`** – Reboot the BMC itself (power-cycles the management controller) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=advanced%20%20Advanced%20node%20modes,will%20lose%20power%20until%20booted)).

Each command has its own syntax, options, and behavior. In the sections below, we document each in detail, including usage examples, flags, and expected outputs or errors.

---

## `power` – Manage Node Power State

**Description:** The `power` command controls the power state of the compute modules (nodes). You can turn nodes on, shut them down (cut power), or reset (reboot) them. There is also a `status` subcommand to query the current power state of all nodes.

**Syntax:** `tpi power <action> [--node N]`

- `<action>` can be one of: `on`, `off`, `reset`, `status`.  
- `--node <N>` (or `-n N`) specifies the target node number (1–4). If omitted, the action may apply to **all nodes** by default ([Automating Node and USB Settings](https://docs.turingpi.com/docs/turing-pi2-automating-node-and-usb-settings#:~:text=chmod%20%2Bx%20%2Fetc%2Finit)). (The CLI originally was designed to handle specific nodes, but note that some older firmware versions only toggled all nodes at once ([Automating Node and USB Settings](https://docs.turingpi.com/docs/turing-pi2-automating-node-and-usb-settings#:~:text=chmod%20%2Bx%20%2Fetc%2Finit)). In current usage, you should include `--node` to target a specific node. If you want to affect all nodes, you can either omit the `--node` or explicitly run the command for each node as needed.)

**Options:** Aside from the global options (host, user, etc.), `power` accepts `--node` to specify the node. There are no other special flags for this command itself.

**Subcommands / Actions:**

- **`on`** – **Power On** the specified node. This supplies power to the compute module so it will boot. If no node is specified, it attempts to power on all nodes ([Automating Node and USB Settings](https://docs.turingpi.com/docs/turing-pi2-automating-node-and-usb-settings#:~:text=chmod%20%2Bx%20%2Fetc%2Finit)).  
- **`off`** – **Power Off** the specified node. This cuts power to the module (equivalent to a hard shutdown). If no node specified, all nodes will be powered off.  
- **`reset`** – **Reset/Reboot** the specified node. This is effectively a power cycle (the BMC will momentarily cut power or pulse the reset line to reboot the module) ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=)). Use this if a node is unresponsive and you need to force a reboot. (If no `--node`, it may reset all nodes, but typically you should always specify a node for reset to avoid unintended restarts.)  
- **`status`** – Query the power state of nodes. This will output each node and whether it’s `on` or `off` ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=%24%20tpi%20power%20status%20,is%20entered%20at%20a%20prompt)). The status command always returns all nodes’ state (ignores the `--node` filter, if given).

**Examples:**

- Turn **on** node 1:  
  ```bash
  tpi power on --node 1
  ```  
  Expected output: none (it will just return to shell if successful). The node’s power LED will light up, and the node will begin booting. If authentication is required, you’ll be prompted or need to supply credentials.

- Turn **off** node 1:  
  ```bash
  tpi power off -n 1
  ```  
  This immediately cuts power. Use with caution – this is like pulling the plug, so the OS on that node will not shut down gracefully.

- **Reset** node 2:  
  ```bash
  tpi power reset -n 2
  ```  
  The node will reboot (power cycle). This is useful if the node is hung or you need to quickly reboot it remotely.

- Get **status** of all nodes:  
  ```bash
  tpi power status
  ```  
  Example output:  
  ```text
  node1: off  
  node2: on  
  node3: off  
  node4: off  
  ```  
  Each node is listed with its current state (on/off) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=%24%20tpi%20power%20status%20,is%20entered%20at%20a%20prompt)).

**Notes and Expected Behavior:**

- Powering on a node that is already on has no effect (it will remain on). Similarly, powering off a node that’s already off does nothing. The CLI may simply return without error in these cases.
- When turning off or resetting a node, any running software on that node will be abruptly stopped (unless you have an OS-level shutdown mechanism). There is typically no confirmation prompt for `off` or `reset`, so be sure you targeted the correct node.
- **Simultaneous Operations:** While you can omit `--node` to affect all nodes, it is often clearer to script individual node commands (especially since firmware differences might interpret an omitted node differently). If you need to power on multiple nodes at once, you can run `tpi power on` in parallel or simply trust that without `--node` it brings up all nodes. The BMC API does allow setting multiple nodes states in one call ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=Set%20power%20status%20of%20specified,nodes)) ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=)), and the CLI aligns with that.
- If a specified node number is out of range (not 1–4), the CLI will return an error (e.g., “invalid node”). Likewise, if you attempt to manage a node that isn’t physically present (empty slot), the command will still execute – the BMC will apply power or cut power to that slot, but obviously no module will boot. No harm is done; you’ll just see that the status remains off (for power on, an empty slot won’t draw power so it might still show “off”).
- In some firmware versions prior to v2.2, the CLI’s per-node power control had a bug where `tpi power on/off` without `--node` would affect all nodes and using `--node` still might have affected all nodes ([Automating Node and USB Settings](https://docs.turingpi.com/docs/turing-pi2-automating-node-and-usb-settings#:~:text=chmod%20%2Bx%20%2Fetc%2Finit)). This has been addressed in newer releases – ensure your BMC firmware and `tpi` tool are up to date. If you find `--node` is not working as expected, update the BMC or use the BMC web UI or API as a workaround ([Automating Node and USB Settings](https://docs.turingpi.com/docs/turing-pi2-automating-node-and-usb-settings#:~:text=chmod%20%2Bx%20%2Fetc%2Finit)).

## `usb` – USB Host/Device Switching

**Description:** The Turing Pi 2 has a shared USB 2.0 OTG bus that can be routed either to one of the compute nodes or to the BMC’s own USB interface. Only one controller at a time can own the USB bus (this includes the mini-PCIe USB lanes for nodes 1 and 2, which are multiplexed with the OTG port) ([Automating Node and USB Settings](https://docs.turingpi.com/docs/turing-pi2-automating-node-and-usb-settings#:~:text=The%20USB%20lanes%20of%20the,Express%20will%20be%20hidden)). The `usb` command lets you switch this USB connection among **device mode** and **host mode** for a given node or to the BMC, or check the current routing status.

In practice, this is used for tasks like flashing a node or using the OTG port: for example, putting a node in *device mode* can allow it to appear as a USB device (mass storage, etc.) to the BMC or a PC, whereas *host mode* allows the node to use the OTG port to connect USB peripherals. The USB-A port on the Turing Pi 2 back panel (if present) is part of this shared bus.

**Syntax:** `tpi usb [OPTIONS] <MODE>`

- `<MODE>` is required and specifies the mode to set or query. It can be:
  - `host` – Route the USB bus to a node in **host** mode (node controls the bus to use USB devices). 
  - `device` – Route the USB bus to a node in **device** mode (node exposes itself as a USB device). 
  - `status` – Query the current USB mode and routing status.
- Options:
  - `--node <NODE>` (or `-n <NODE>`) – Specify which node (1–4) to route the USB to ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=Options%3A%20,If%20unused%2C%20an)). *This option is required* for `host` or `device` modes (to know which node to apply to). It is ignored for `status` (status will report whichever node is currently wired). 
  - `--bmc` (or `-b`) – Instead of routing to a node, route the USB bus to the **BMC** itself ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=Options%3A%20,If%20unused%2C%20an)). This option is mutually exclusive with `--node`. Use `-b` if you want the BMC to take control of the USB bus (either to act as a USB host or device, depending on mode).  
  - *(Global options like `--host`, `--user`, etc., are also accepted as usual ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=,a%20cached%20token%20file%20is)).)*

**Usage Modes:**

- **`tpi usb host --node N`** – Connect the USB bus to **node N in host mode**. The specified node will be able to use the USB-A (OTG) port as a host, e.g. to recognize USB devices (flash drives, etc.). For example, if you want node 3 to use the shared USB port for peripherals, run `tpi usb host -n 3`. This will typically disconnect any other node from the bus. Note that by default on cold boot, Node 1 is set to host mode ([Automating Node and USB Settings](https://docs.turingpi.com/docs/turing-pi2-automating-node-and-usb-settings#:~:text=On%20a%20cold%20boot%20of,1%20that%20use%20USB%20unavailable)).
- **`tpi usb device --node N`** – Connect the USB bus to **node N in device mode**. This effectively exposes node N to the BMC’s USB, often used to flash the node or access it as a gadget (mass storage, etc.). For instance, `tpi usb device -n 1` will disconnect the bus from others and attach node1’s OTG interface to the BMC/USB port in device mode (e.g., node1 might appear as a USB device to a PC connected to the Turing Pi’s USB port).
- **`tpi usb host --bmc`** – Route the USB bus to the **BMC in host mode** ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=Options%3A%20,If%20unused%2C%20an)). In this mode, the BMC controls the USB-A port (so you could plug a peripheral into the Turing Pi’s USB port and the BMC’s Linux can use it).
- **`tpi usb device --bmc`** – Route the USB bus to the **BMC in device mode**. The BMC will act as a USB device on the OTG port (less commonly used – an example would be exposing the BMC’s storage as a USB gadget to an external PC).
- **`tpi usb status`** – Display which mode and which controller currently has the USB bus. The output will indicate the mode (Device or Host) and which node or BMC is active ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=)). For example, it might output something like: “`mode: host, node: 3, route: USB-A`” meaning node3 is using the USB-A port in host mode.

**Examples:**

- Switch USB control to **Node 2 as host** (so Node 2 can use a keyboard or other USB device):  
  ```bash
  tpi usb host --node 2
  ```  
  Output: (none, unless there’s an error or you query status after). Internally, the BMC configures the USB switch such that Node2’s OTG is connected to the physical USB port.

- Switch USB control to **Node 1 as device** (for flashing Node 1 from the BMC or an external PC):  
  ```bash
  tpi usb device -n 1
  ```  
  After this, Node1’s eMMC (if it is a Raspberry Pi CM4, for example) might enumerate as a USB mass storage device accessible to the BMC or a connected PC. 

- Give the USB port to the **BMC** (perhaps you want the BMC to use a USB drive for logging or update):  
  ```bash
  tpi usb host -b
  ```  
  Now the BMC’s OS sees the USB port. 

- **Check USB status:**  
  ```bash
  tpi usb status
  ```  
  Example output:  
  ```json
  {
    "mode": "Device",
    "node": 1,
    "route": "USB-A"
  }
  ```  
  This indicates Node1 is in Device mode on the USB-A port (meaning likely Node1 is currently in flashing/MSD mode) ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=Returns%3A)). In plain text mode, it might print a concise line instead of JSON unless `--json` is used.

**Important Notes:**

- Only one node can have the USB bus at a time. If you switch to another node or to BMC, any previous connection is automatically dropped. For instance, if Node1 was in host mode and you issue `tpi usb host -n 3`, Node1’s USB connection is disconnected and Node3 takes over.
- The **Mini PCIe slots for Node1 and Node2** share this USB bus. If Node1 or Node2 is using the USB in host/device mode via `tpi usb`, **any USB-based Mini PCIe device on that node will be hidden or disconnected** ([Automating Node and USB Settings](https://docs.turingpi.com/docs/turing-pi2-automating-node-and-usb-settings#:~:text=The%20USB%20lanes%20of%20the,Express%20will%20be%20hidden)). For example, if Node1 has a Mini PCIe WiFi card that uses USB, and you set `tpi usb host -n 1`, that card may not function because the USB lines are switched to the OTG port. Plan your USB routing accordingly depending on which nodes need their MiniPCIe devices.
- **Default on Boot:** On a fresh power-up of the Turing Pi 2, the default setting is typically **Host mode on Node1** ([Automating Node and USB Settings](https://docs.turingpi.com/docs/turing-pi2-automating-node-and-usb-settings#:~:text=On%20a%20cold%20boot%20of,1%20that%20use%20USB%20unavailable)). This means Node1 has the USB bus by default. If you need a different node to use the USB bus automatically, you can change it (for example, some users create an init script on the BMC to set Node3 as host at startup ([Automating Node and USB Settings](https://docs.turingpi.com/docs/turing-pi2-automating-node-and-usb-settings#:~:text=A%20simple%20startup%20script%20can,directory%20with%20the%20following%20content))). 
- **Flashing Nodes:** When using the `tpi flash` command (documented below) to flash an OS image to a node, the CLI will usually handle switching the USB to device mode for that node automatically. But if doing manual procedures (like using Raspberry Pi’s `rpiboot` utility or mounting a node’s storage), you may need to manually set `tpi usb device -n <node>` to put the node in the right mode.
- If you issue an incorrect combination (e.g., `tpi usb host` without either `--node` or `--bmc`), the CLI will respond with a help or error message, since it won’t know where to route. For example, running `tpi usb` with missing arguments will print the usage info ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=%24%20tpi%20usb%20Change%20the,routed%20to%20one%20node%20simultaneously)) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=Options%3A%20,If%20unused%2C%20an)). Always specify either a node or `-b` for actual mode changes.
- After changing USB routing, you might want to power cycle or reboot a node depending on context. For instance, to use the Raspberry Pi Compute Module in device (flashing) mode, you often need to have the node unpowered, set `device` mode, then power it on so it boots into that mode where it appears as USB storage.

## `firmware` – BMC Firmware Upgrade

**Description:** The `firmware` command is used to upgrade the Turing Pi 2’s BMC firmware. This updates the software running on the BMC (the Allwinner T113-S3 module). Use this command to apply official firmware updates (which may add features or bug fixes to the BMC). Under the hood, this typically involves providing a firmware file (often with extension `.tpu` for Turing Pi Update) and initiating the upgrade process. 

**Syntax:** `tpi firmware [OPTIONS] <firmware_file>`

The exact syntax can vary with CLI version, but generally you provide the path to the new firmware file. For example: 

```bash
tpi firmware /path/to/firmware-v2.2.1.tpu
``` 

**Options:** There may not be many sub-options besides global ones. Potentially:
- Some versions might allow a `--file <path>` or simply taking the file path as a positional argument. 
- The CLI might also allow an interactive prompt to confirm upgrade (or not, depending on design).

**Usage & Examples:**

1. **Download the Firmware:** First, obtain the firmware image from the official source (e.g., a `.tpu` file from Turing Pi’s website). Ensure it’s accessible to the machine where you run `tpi`. 

2. **Run the Upgrade:**  
   ```bash
   tpi firmware ~/Downloads/turingpi2_v2.2.1.tpu
   ```  
   The CLI will likely upload this file to the BMC and trigger the update. You might see progress output or a message like “Firmware upgrade in progress.”

   *Example output:* The CLI might print something like: “Uploading firmware... Flashing... Success, rebooting BMC.” (Exact wording depends on implementation.)

3. **BMC Reboot:** After a successful firmware flash, the BMC will typically reboot automatically to apply the new firmware. **Important:** During this BMC reboot, you will lose connection to the BMC (and the nodes will lose power temporarily since the BMC controls their power) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=advanced%20%20Advanced%20node%20modes,will%20lose%20power%20until%20booted)). The CLI might even forcibly reboot the BMC as part of the process. Expect your SSH or `tpi` connection to drop. You may need to reconnect after a minute or so when the BMC comes back up.

**Prerequisites & Warnings:**

- **Version Gaps:** If your BMC is running a very old firmware (v1.x) and you’re upgrading to a v2.x, there might be special steps (often involving using an SD card). The `tpi firmware` command is primarily for updating within the same major version (like 2.x to a newer 2.x) when the BMC already supports OTA updates. The official docs note that to upgrade from pre-2.0 to 2.0+, you must use an SD card method ([Turing-pi BMC firmware - GitHub](https://github.com/turing-machines/BMC-Firmware#:~:text=Turing,Note%202%3A%20Prior%20to%20v2)). Once on 2.x, you can use this CLI for subsequent updates.
- **File Size Limitations:** Earlier firmware (e.g., v2.0.0) had an issue where the BMC’s update service couldn’t handle files >50MB ([Flashing OS](https://docs.turingpi.com/docs/raspberry-pi-cm4-flashing-os#:~:text=%3E%20,the%20issue%20has%20been%20resolved)). If your firmware file is large, ensure you have a recent `tpi` and BMC version that supports it. If not, you might get an error or the upgrade may fail. This was acknowledged as a known limitation and subsequent updates aimed to fix it ([Flashing OS](https://docs.turingpi.com/docs/raspberry-pi-cm4-flashing-os#:~:text=,the%20issue%20has%20been%20resolved)).
- **Network**: The command uses your network connection to send the firmware to the BMC. Do not interrupt this process. A failure during firmware flashing could leave the BMC in a bad state. However, the BMC has a failsafe mechanism (booting from SD) in case of bad flash ([Upgrade Firmware correctly Turing Pi # forum](https://forum.turingpi.com/t/26916359/upgrade-firmware-correctly#:~:text=Upgrade%20Firmware%20correctly%20Turing%20Pi,and%20booting%20from%20that)).
- **No Concurrency:** Do not try to run multiple `tpi firmware` upgrades at once. Only one upgrade should happen at a time, and ideally, ensure no other heavy tasks on the BMC during the update.

**Possible Outputs and Errors:**

- If you provide no file or an incorrect usage, you’ll see a help message for the firmware command (similar to other commands).
- If the firmware file path is wrong or inaccessible, the CLI will report **“file not found”** or **“cannot open file”**.
- If using an unsupported older BMC (like running from SD boot mode where OTA isn’t possible), the CLI may report that firmware upgrade isn’t supported in that mode.
- On success, the CLI might not explicitly say “success” – it could just drop connection when the BMC reboots. You should then verify the new firmware version after reconnecting (using `tpi info` or checking the BMC UI, which shows the firmware version).

**After Upgrade:** Once the BMC reboots, you may need to re-authenticate (especially if the token cache was invalidated or if the API version changed). Use `tpi info` to confirm the BMC’s new version (the `info` output includes the BMC daemon version). Also, some firmware upgrades might reset settings, so ensure things like network (hostnames, etc.) are still as expected.

## `flash` – Flash OS Image to a Node

**Description:** The `flash` command allows you to write an operating system image to one of the compute module’s storage (e.g., the eMMC on a Raspberry Pi CM4 or Turing RK1). This provides a way to program the nodes without removing them – essentially an OTA flash using the BMC’s USB OTG mechanism. The CLI streams a disk image file from your computer (or from the BMC, if run there) to the target node by putting that node in device mode and writing the image to its storage.

**Syntax:** `tpi flash --node <N> --image <file>` (or using short flags `-n` and `-i`)

- `--node <N>` / `-n <N>` – Specify the node number (1–4) to flash ([Turing RK1 | TALOS LINUX](https://www.talos.dev/v1.9/talos-guides/install/single-board-computers/turing_rk1/#:~:text=Flash%20the%20image%20to%20the,of%20the%20Turing%20Pi%202)). This is required.
- `--image <file>` / `-i <file>` – Path to the OS image file (e.g., an `.img` or `.raw` disk image) to be flashed ([Turing RK1 | TALOS LINUX](https://www.talos.dev/v1.9/talos-guides/install/single-board-computers/turing_rk1/#:~:text=Flash%20the%20image%20to%20the,of%20the%20Turing%20Pi%202)) ([Turing Pi 2 Home cluster - DEV Community](https://dev.to/tomassirio/turing-pi-2-home-cluster-5edc#:~:text=The%20Turing%20Pi%202%20has,some%20stats%2C%20and%20so%20on)). This file could be a Raspberry Pi OS image, an Ubuntu image for Jetson, a Talos Linux image, etc. Make sure it’s uncompressed (e.g., `.img` or raw disk file, not a .zip or .xz unless the CLI supports on-the-fly decompression).
- *(Global options like `--host`, `--user` as needed for connecting and auth.)*

**Example Usage:**

- Flash a Raspberry Pi OS image to **Node 1**:  
  ```bash
  tpi flash -n 1 -i ~/Downloads/raspios_lite_arm64.img
  ```  
  This will initiate the process of writing the image file to Node1’s eMMC. You might see progress output or a simple blinking cursor for a while. Once done, you should see a message or prompt return. After flashing, you would typically power on the node (if it’s not already on) or reset it to boot into the new OS.

- Flash a Talos Linux image to **Node 4 (RK1)** as shown in an example:  
  ```bash
  tpi flash -n 4 -i metal-arm64.raw ([Turing RK1 | TALOS LINUX](https://www.talos.dev/v1.9/talos-guides/install/single-board-computers/turing_rk1/#:~:text=Flash%20the%20image%20to%20the,of%20the%20Turing%20Pi%202))
  tpi power on -n 4        # then power it on to boot the new image ([Turing RK1 | TALOS LINUX](https://www.talos.dev/v1.9/talos-guides/install/single-board-computers/turing_rk1/#:~:text=Flash%20the%20image%20to%20the,of%20the%20Turing%20Pi%202))
  ```  
  In this case, they used a raw disk image for Talos and then turned the node on.

**What to Expect:**

- When you run `tpi flash`, the CLI will likely:
  1. Power off the target node if it’s on (since flashing often requires it in a particular mode). In some cases, it might reset it into a special mode automatically.
  2. Switch the USB to connect that node in device mode to the BMC (similar to doing `tpi usb device --node N` behind the scenes).
  3. Transfer the image file data to the node. This could take a few minutes depending on the image size (e.g., a few hundred MB). There might be a progress indicator (some versions could show a percentage or just print when done).
  4. Once completed, the node might be left in device mode. The CLI might or might not automatically power the node back on. Often, you will manually power it on after flashing (as shown in the example) ([Flashing OS](https://docs.turingpi.com/docs/raspberry-pi-cm4-flashing-os#:~:text=Stop%20Power%20to%20Node1%3A)).
- During the flash, the CLI may output nothing until it finishes (don’t interrupt it!). Some implementations stream data over the network to the BMC which then writes it to the eMMC; a lack of output is normal during the transfer. High CPU or network usage on your PC and BMC during this time is expected.

**Important Considerations:**

- **Size and Speed:** Flashing over the network can be slower than using an SD card directly. The BMC uses a USB 2.0 connection to the node in device mode, so write speeds might be ~20-30 MB/s at best. A 8GB image could take several minutes. Ensure your PC doesn’t sleep and maintain the network connection.
- **File Size Limit:** As noted earlier, older firmware had a 50MB limit bug ([Flashing OS](https://docs.turingpi.com/docs/raspberry-pi-cm4-flashing-os#:~:text=,the%20issue%20has%20been%20resolved)). Most OS images are larger (Raspberry Pi OS ~2GB, etc.), so make sure you have the BMC firmware updated beyond that bug. If not, the flash command might fail or be “unuseful” ([Flashing OS](https://docs.turingpi.com/docs/raspberry-pi-cm4-flashing-os#:~:text=,the%20issue%20has%20been%20resolved)).
- **Supported Node Types:** This works for Raspberry Pi Compute Module 4 and Turing RK1 modules (and likely Jetson modules if they support USB device flashing). Note that some modules (like Jetson) might require using their own flashing utilities instead of this method. The `tpi flash` is most commonly used for Raspberry Pi CM4 and similar which support USB mass storage mode. For Jetson Orin/Nano, you may need to use Nvidia’s tools (the BMC doesn’t automatically handle Jetson flashing in current firmware, though future updates might integrate it).
- **After Flashing:** You should power cycle the node to exit the device mode. For example, do: `tpi power off -n 1` (if it wasn’t already off), then `tpi power on -n 1` to boot the freshly flashed OS ([Accessing nodes' filesystems](https://docs.turingpi.com/docs/tpi-accessing-nodes-filesystems#:~:text=6,n%201)). Ensure the node boots and check its output (you can use `tpi uart get` to watch boot logs, see UART section).
- **Multiple Nodes:** Flash nodes one at a time. If you need to clone an image to all nodes, do them sequentially: flash node1, then node2, etc. The `tpi flash` command currently handles one node at a time (the BMC likely cannot flash multiple nodes concurrently due to the single USB bus limitation).
- **Error Handling:** If the flash process fails (CLI might report an error), the node might remain in device mode and not have a valid OS. You can simply retry the flash. If a failure happened mid-way, you should probably power cycle the node and ensure it’s in a proper state (it may appear as an empty/uninitialized USB device until re-flashed). The CLI might give errors like “Failed to write” or “Timeout” if something goes wrong (like a network drop). Ensure stable connectivity and adequate power (flashing draws power, make sure your power supply to Turing Pi is sufficient).

**Output:** On success, some versions of `tpi` may just silently finish and return to prompt. Others might say “Flash complete” or similar. You will not explicitly see a “Verification” unless the tool does it internally. It’s a good idea to boot the node and test the OS to verify the flash was successful.

## `eth` – Ethernet Switch & Network Configuration

**Description:** The `eth` command manages the on-board Ethernet switch and network interfaces of the Turing Pi 2. The Turing Pi 2 board has two physical Gigabit Ethernet ports and an integrated switch that connects the 4 nodes, the BMC, and the external network. By default, the Ethernet ports are in **bridge mode** ([Specs and I/O Ports](https://docs.turingpi.com/docs/turing-pi2-specs-and-io-ports#:~:text=Ethernet%20Ports%202x%201Gbps%20Ethernet,small%20Molex%20Fan%20Power%20Connector)) – typically meaning the BMC and nodes all share a network bridge. This command allows you to view and modify the switch configuration, such as resetting the switch or changing networking modes (bridged or isolated).

**Functions of `eth` command:**

- **View Network Info:** Likely there is a subcommand to show the status of network interfaces and switch (e.g., IP addresses of BMC, link status of ports).
- **Reset Switch:** You can reset or restart the Ethernet switch chip if needed (equivalent to the “Reset Network” button in the web UI ([BMC User Interface](https://docs.turingpi.com/docs/turing-pi2-bmc-ui#:~:text=match%20at%20L237%20Reset%20Network))).
- **Change Bridge Mode:** If supported, toggle the bridge that links BMC with the switch. For example, you might disable bridging so that one of the Ethernet ports is dedicated to the BMC and the other to the node network separately.

**Syntax:** The exact subcommands for `eth` may include: `tpi eth status`, `tpi eth reset`, and possibly `tpi eth bridge [on|off]` or similar.

Common usage patterns: 

- `tpi eth status` – Show the current network setup (which ports are up, BMC’s IP, etc.).
- `tpi eth reset` – Restart the Ethernet switch. This will momentarily drop network connectivity for nodes (and possibly the BMC if it’s using the switch), and then restore it ([BMC User Interface](https://docs.turingpi.com/docs/turing-pi2-bmc-ui#:~:text=The%20reset%20network%20button%20can,the%20top%20of%20the%20page)). Use this if the switch is unresponsive or after changing network configs.
- `tpi eth mode <mode>` – There might be a mode setting. For instance, `mode bridge` vs `mode split`. (In “bridge” mode, both RJ45 ports act as one network; in a hypothetical “split” mode, maybe one port is for BMC exclusively and one for node switch uplink – if the hardware supports such configuration.)

**Examples:**

- **Check network status:**  
  ```bash
  tpi eth status
  ```  
  *Expected output:* A listing of network interfaces and maybe switch status. For example:  
  ```
  eth0: up, IP 192.168.1.50, MAC 12:34:56:78:9A:BC (BMC interface)
  br0: up, IP 192.168.1.50, members: eth0, sw0
  sw0: up, linked, (internal switch interface for nodes)
  ```  
  This is an illustrative example; actual output might differ. The key info is the BMC’s IP/MAC and possibly that a bridge exists.

- **Reset the Ethernet switch:**  
  ```bash
  tpi eth reset
  ```  
  After running this, the network switch is rebooted. You might lose ping/SSH to the BMC for a few seconds (approximately 5 seconds per docs) ([BMC User Interface](https://docs.turingpi.com/docs/turing-pi2-bmc-ui#:~:text=The%20reset%20network%20button%20can,the%20top%20of%20the%20page)). The CLI should report success or simply return. In the web UI, a “success” toast is shown; on CLI you might just get back to prompt if successful.

- **Disable bridging (hypothetical):**  
  ```bash
  tpi eth bridge off
  ```  
  *(If supported)* This would separate the BMC’s network from the node network. One physical port would remain for nodes (switch to nodes only), and the other port would be BMC only. This could be useful for isolating management traffic. After doing this, the BMC might only be accessible via one port and the nodes via the other. To re-enable: `tpi eth bridge on`.

**Notes:**

- **Dual Ethernet Ports:** The Turing Pi 2 has 2x 1 Gbps Ethernet ports on the board ([Specs and I/O Ports](https://docs.turingpi.com/docs/turing-pi2-specs-and-io-ports#:~:text=Ethernet%20Ports%202x%201Gbps%20Ethernet,small%20Molex%20Fan%20Power%20Connector)). By default in **bridge mode**, these likely act like a two-port switch uplink: you can use either port to connect to your upstream network, and BMC and all nodes will be reachable on that same LAN (they effectively share the connection). If you connect both ports to a network, be careful of network loops (unless STP is running). Typically, you would use one port at a time unless doing a fancy network config.
- **Bridge Mode vs Separate:** In some firmware updates, the network was reworked to use Linux bridging (interface `br0` on the BMC). The BMC’s primary NIC (`eth0`) and the switch’s uplink (`sw0`) are part of `br0`. The `eth` command might allow breaking this bridge. For example, if you wanted the BMC on a separate management network, you could remove it from the bridge. This is advanced usage and not commonly needed; if unsupported via CLI, it could require manual config on the BMC (editing network config files).
- **When to Reset Switch:** If you change VLAN settings or encounter a situation where some nodes can’t communicate over Ethernet, resetting can help. Also, if you script changes (like moving the BMC off the bridge), you might need to reset or reconfigure the switch.
- **Potential Error Messages:** If the command is unrecognized (e.g., you type `tpi eth restart` instead of `reset`), it will show the help for `tpi eth`. If the switch fails to reset properly, you might not get an error but you may notice network issues persist. Typically, the operation is simple and either works or the BMC will log something.
- **VLANs and Advanced Config:** The CLI `eth` command in its initial version likely doesn’t expose VLAN configurations. The nodes are all in the same layer2 domain by default. If you need to isolate nodes from each other at L2, that might require external network configuration or future firmware that supports it. Keep an eye on official updates for any new subcommands under `eth` that manage VLAN tagging per node port, etc.

In summary, use `tpi eth` to view and manage the cluster’s network topology. Most home users will not need to use `eth` often, except possibly `status` to get IP info or `reset` if something goes wrong.

## `uart` – Node Serial Console Access

**Description:** The `uart` command provides access to the serial console (UART) of the compute modules via the BMC. Each node’s UART (the Linux console on Raspberry Pi, for instance) is wired to the BMC, and this CLI can read from or write to that serial interface ([UART](https://docs.turingpi.com/docs/tpi-uart#:~:text=With%20,things%20to%20be%20aware%20of)). This is useful for monitoring boot logs, interacting with the bootloader, or logging in to a node’s OS via serial (especially if network is not yet configured or the node is inoperative otherwise).

**Important:** UART on many OS (like Raspberry Pi OS) is **disabled by default** for login ([UART](https://docs.turingpi.com/docs/tpi-uart#:~:text=things%20to%20be%20aware%20of%3A)). You may need to enable it on the node’s OS (for Raspberry Pi, by adding `enable_uart=1` in `/boot/config.txt` or using `raspi-config`) ([UART](https://docs.turingpi.com/docs/tpi-uart#:~:text=Enabling%20UART%20in%20Raspberry%20Pi,OS)) ([UART](https://docs.turingpi.com/docs/tpi-uart#:~:text=,Unmount%20partition)). Ensure the node’s UART is enabled to get console output.

**Syntax:** `tpi uart [OPTIONS] <action>`

- `<action>` can be `get` or `set`. 
  - `get` – Read from the node’s UART buffer (i.e., retrieve any output the node has sent over serial). This will output any new data since last read and then clear the buffer ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=%60%2Fapi%2Fbmc%3Fopt%3Dset%26type%3Duart%26cmd%3Decho)) ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=)).
  - `set` – Send data to the node’s UART (i.e., input to the node’s console) ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=)).
- Options:
  - `--node <N>` / `-n <N>` – Specify the node number whose UART to use (1–4). Required, as you must indicate which node’s console you want.
  - For `set` action: `--cmd <string>` – The command or text to send over serial to the node ([UART](https://docs.turingpi.com/docs/tpi-uart#:~:text=And%20the%20latter%20writes%20data%2C,effectively%20executing%20a%20command)). This can be a single command like `root` (to send the username “root”) or a longer string. If the string has spaces or special chars, quote it.
  - There is also an optional `encoding` parameter in the BMC API (UTF-8 default) ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=Read%20buffered%20UART%20data%2C%20clearing,output%20buffer)), but the CLI likely handles this automatically. Unless you have special character needs, you won’t need to specify encoding.

**Examples:**

- **Read console output from Node 1:**  
  ```bash
  tpi uart --node 1 get
  ```  
  This will fetch any available serial output from node1. If the node has just booted, this might include the kernel boot messages and login prompt. Example output snippet:  
  ```text
  [    5.610444] systemd[1]: Started Journal Service.
  Debian GNU/Linux 11 raspberrypi ttyS0
  raspberrypi login:
  ```  
  (This indicates the node booted Raspberry Pi OS and is at a login prompt) ([UART](https://docs.turingpi.com/docs/tpi-uart#:~:text=%24%20tpi%20uart%20,11%20raspberrypi%20ttyS0%20raspberrypi%20login)). Each call to `get` will return new data; if you call it again and nothing new was printed by the node, it may return nothing or an empty result.

- **Send input to Node 1’s console:** Suppose the node is at a login prompt, and you want to log in.  
  ```bash
  tpi uart --node 1 set --cmd 'root'
  ```  
  This sends “root” (plus a newline, likely) to node1’s UART, as if you typed “root” on a keyboard at its console ([UART](https://docs.turingpi.com/docs/tpi-uart#:~:text=%24%20tpi%20uart%20,1%20get%20echo%20hi%20hi)). Then you might send the password:  
  ```bash
  tpi uart -n 1 set --cmd 'your_password'
  ```  
  (Replace `your_password` with the actual password, e.g., `turing` if using default on a fresh DietPi, etc.) Note: when sending the password, you won’t see it on output for security, but it’s being sent.

  After sending login credentials, you can use `get` again:  
  ```bash
  tpi uart -n 1 get
  ```  
  to see the login result. If successful, you might capture the welcome message and shell prompt. You can then send commands, e.g.:  
  ```bash
  tpi uart -n 1 set --cmd 'echo "hi"' 
  ```  
  and then `get` to see the output of that command ([UART](https://docs.turingpi.com/docs/tpi-uart#:~:text=%5B..%5D%20Linux%20raspberrypi%205.15.84,1%20get%20echo%20hi%20hi)). In this example, the node printed back the command and “hi”.

- **Monitoring boot:** You can spam `tpi uart -n X get` in a loop or simply use it after turning on a node to retrieve its boot messages. For an interactive real-time view, you might prefer using `picocom` directly on the BMC (the BMC’s OS has `picocom` installed for interactive serial sessions) ([UART](https://docs.turingpi.com/docs/tpi-uart#:~:text=,installed%20on%20the%20BMC)) ([UART](https://docs.turingpi.com/docs/tpi-uart#:~:text=Using%20)). The `tpi uart` method is non-interactive (one command at a time). For continuous monitoring, running `tpi uart ... get` repeatedly or using the web UI’s console (if available) is needed.

**Notes:**

- **Buffered Output:** The BMC firmware buffers the UART output from each node. Using `get` will retrieve all buffered data and then clear the buffer ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=)). So if you run `get` too infrequently, you might miss data (if the buffer overflowed) or if you run it too often, you might get small chunks. A best practice is to run it in a loop or use proper tools for continuous reading.
- **No Interactive Terminal Emulation:** The `tpi uart set/get` is essentially an I/O pipe. There is no terminal UI – it won’t behave exactly like a live console where you see characters as you type. The pattern is typically: send a whole command or credential with `set`, then use `get` to see everything that came from the node. If you need interactive real-time control (e.g., to use an interactive installer), consider using `ssh` or connecting a serial terminal to the BMC and using `picocom` on the BMC for that node.
- **Enabling UART on Node OS:** As mentioned, ensure the OS is configured for serial console. Raspberry Pi OS requires `enable_uart=1` in `/boot/config.txt` ([UART](https://docs.turingpi.com/docs/tpi-uart#:~:text=Enabling%20UART%20in%20Raspberry%20Pi,OS)). Some Linux distros might have a serial console getty running by default on certain SoCs, others not. If you see nothing in `get` after booting, the node might not be outputting to that UART (check if TX line is enabled or if console=ttyS0 is set in its boot args). The documentation outlines ways to enable it, including editing the config via mounting the node’s boot partition (which you can do using the `advanced msd` mode, see below) ([UART](https://docs.turingpi.com/docs/tpi-uart#:~:text=,Unmount%20partition)).
- **UART Device Mapping:** Note that the BMC’s UART device paths for nodes might not be intuitive (they aren’t simply ttyS0–ttyS3 in order). The mapping changed between firmware versions ([UART](https://docs.turingpi.com/docs/tpi-uart#:~:text=%2A%20Determine%20the%20serial%20pseudo,of%20the%20node)). But using `tpi uart` abstracts that – you just specify node number.
- **Multiple Nodes:** You can simultaneously open multiple consoles by running separate shell instances of `tpi uart ... get` for different nodes, but the CLI itself isn’t interactive. If you do so, remember each call needs to authenticate (unless token cached) which might slow down rapid polling. For heavy debug logging, you might directly use `picocom` on the BMC as it’s more real-time.
- **Errors:** If you try to `get` from a node and see no response, it’s not necessarily an error – it might just be no output available. If the node number is wrong, you’ll get an error about invalid node. If you omit `get/set`, the CLI will show usage. If you omit `--cmd` on a `set`, it will likely error that a command string is needed.
- **Line Endings:** `tpi uart set` likely automatically appends a newline (`\n`) at the end of the provided `--cmd` string (so that it acts like an Enter key press). If you find that it’s not, you might need to include `\r` or `\n` explicitly. In most cases, providing the command text is enough.

UART access is a powerful feature for low-level troubleshooting – for example, if your node’s network is misconfigured, you can use UART to get in and fix it. It’s essentially like having a monitor+keyboard directly on the Pi/Jetson.

## `advanced` – Advanced Node Modes (e.g. USB Mass Storage Boot)

**Description:** The `advanced` command provides access to special modes or features for the compute modules that go beyond standard operation. Currently, the primary mode available is **“MSD” (Mass Storage Device) mode**, which reboots a node into a mode where it exposes its eMMC/storage as a USB device to the BMC or a host PC ([Accessing nodes' filesystems](https://docs.turingpi.com/docs/tpi-accessing-nodes-filesystems#:~:text=With%20,MSD)). This is particularly useful for accessing a node’s filesystem directly or flashing it using standard PC tools.

Future advanced modes may include things like recovery mode for Jetson, etc., but as of now, **`msd`** is the known mode.

**Syntax:** `tpi advanced <mode> --node <N>`

- `<mode>`:  
  - `msd` – Mass Storage Device mode. Puts the node into USB mass storage boot. The node will reboot and its storage will be accessible via USB. 
  - *(Other modes may be added in the future; for example, hypothetically `recovery` for Jetson or others, but not in current documentation.)*
- `--node <N>` / `-n <N>` – the node number to apply the advanced mode to. This is required because you need to specify which node to affect.

**What MSD Mode Does:** When you execute `tpi advanced msd --node X`, the BMC will *power-cycle node X into a special mode* where the node’s eMMC (or SD card, if that’s what it uses) is connected to the BMC’s USB as a mass storage device ([add support for rockchip USB MSC · Issue #137 - GitHub](https://github.com/turing-machines/BMC-Firmware/issues/137#:~:text=The%20Turing%20Pi%20can%20expose,1%20and%20load%20the)). Essentially, the node doesn’t boot into its OS; instead, it enters a bootloader mode that makes its storage accessible externally. The BMC will then enumerate it as a block device (e.g., on the BMC you might see `/dev/sda` appear corresponding to node’s drive). This allows the BMC (or your PC, if you route the USB to PC) to read/write the node’s filesystem.

**Example – Accessing Node Filesystem:**

1. Run MSD mode on Node1:  
   ```bash
   tpi advanced msd --node 1 ([Accessing nodes' filesystems](https://docs.turingpi.com/docs/tpi-accessing-nodes-filesystems#:~:text=With%20,MSD))
   ```  
   Output: nothing major, but the command triggers Node1 to reboot. Give it ~10 seconds ([Accessing nodes' filesystems](https://docs.turingpi.com/docs/tpi-accessing-nodes-filesystems#:~:text=1,vi%20%2Fmnt%2Fraspios%2Fconfig.txt)). The node will not boot its OS; instead, the BMC should now detect Node1’s eMMC as a USB gadget.

2. On the **BMC**, a new device like `/dev/sda` appears. You can SSH into the BMC and mount it. For example:  
   ```bash
   ssh [email protected]   # connect to BMC  
   mkdir /mnt/node1  
   mount /dev/sda1 /mnt/node1      # assuming sda1 is the first partition  
   ```  
   Now you can access files. (If you prefer, you could also route the USB to your PC with `tpi usb device -n 1` *before* msd, but typically BMC access is fine.)

   The docs illustrate this process: after `tpi advanced msd -n 1`, wait, then mount partitions ([Accessing nodes' filesystems](https://docs.turingpi.com/docs/tpi-accessing-nodes-filesystems#:~:text=3,n%201)). For example, mount the boot partition to edit `config.txt` if needed ([UART](https://docs.turingpi.com/docs/tpi-uart#:~:text=,Unmount%20partition)).

3. Perform your edits or read data. For example, you might edit `/mnt/node1/etc/hostname` or copy some log files.

4. When done, **unmount** the partitions:  
   ```bash
   umount /mnt/node1
   ```  
   Then take the node out of MSD mode by rebooting it normally. Since the node was not actually “on” in the normal sense (it was in a special mode), you should do:  
   ```bash
   tpi power off -n 1  
   tpi power on -n 1
   ```  
   This turns the node off and on, so it will boot normally from its storage ([Accessing nodes' filesystems](https://docs.turingpi.com/docs/tpi-accessing-nodes-filesystems#:~:text=6,n%201)). Alternatively, the BMC might automatically turn it off when exiting MSD, but best to explicitly do it.

**Multiple Nodes in MSD:** Originally, only one node at a time could be in MSD mode. A recent firmware (v2.5) added the ability to do it for all 4 simultaneously ([Turing Pi on X: "@mcodec We can. Use the command "tpi advanced ...](https://twitter.com/turingpi/status/1842173336605847731#:~:text=Turing%20Pi%20on%20X%3A%20,modules%20at%20the%20same%20time)), though practicality of accessing four USB drives through one OTG might vary. In any case, you can run `tpi advanced msd -n 2` for node2, etc., sequentially or concurrently if supported. Each will appear as a different device (likely sda, sdb, etc.) on the BMC.

**Other Advanced Modes:** If none are documented, we assume `msd` is the main one. For Jetson modules, the advanced mode for flashing is typically NVIDIA’s recovery – at time of writing, the BMC did not expose a separate mode for that via `tpi` (Jetson flashing is done by toggling a jumper and using NVIDIA tools). Should future updates allow something like `tpi advanced recovery -n <X>` for Jetson, that would be similar concept.

**Notes and Warnings:**

- **Data Safety:** When in MSD mode, treat the node’s storage like an external drive. Unmount cleanly to avoid corruption. Also, **do not boot the node’s OS while it’s mounted on BMC** – that’s why we keep it powered off except when intentionally in this mode.
- **Exiting MSD Mode:** The node stays in the MSD mode until reset. If you accidentally leave a node in MSD mode and just do `tpi advanced msd -n 1` again, it may not toggle off. The correct way is to power cycle the node (off then on) to have it boot normally. The BMC’s `power status` might still show it “on” during MSD (since technically it is powered in that mode). So don’t be confused – just issue an off then on.
- **Compatibility:** The MSD mode for Raspberry Pi CM4 works (it uses the ROM bootloader’s rpiboot mechanism). For Turing RK1 (Rockchip) modules, MSD support was not initially implemented ([add support for rockchip USB MSC · Issue #137 - GitHub](https://github.com/turing-machines/BMC-Firmware/issues/137#:~:text=add%20support%20for%20rockchip%20USB,1%20and%20load%20the)) – the BMC might not be able to put an RK1 into USB bulk mode out of the box (there were feature requests to add it). Check release notes if RK1 support was added for MSD in newer firmware.
- **Use Cases:** This is extremely handy to fix configuration issues (enable UART, change static IP config, expand filesystem, etc.) without pulling out the module. It’s also a method to flash the OS using standard PC imaging tools if `tpi flash` had limitations – e.g., you could put node in MSD and then run Raspberry Pi Imager on your PC to flash it, since the node appears as a USB drive to your PC.
- **Errors:** If you run `tpi advanced msd` on a node and nothing happens, ensure the node was powered on (the CLI might power-cycle it automatically though). If the node doesn’t enter MSD, it could be an unsupported module type or a very early firmware. The CLI should respond with something (maybe “unsupported” if that’s the case). Also, running it on an already MSD-mode node could just do another reset – watch for multiple devices or none.

In summary, the `advanced msd` mode is a powerful feature for maintenance. Use it carefully, and always restore the node to normal operation after.

## `info` – System Information

**Description:** The `info` command prints detailed information about the Turing Pi 2 board and BMC. It aggregates various data points such as network interfaces (IP and MAC of BMC, possibly of nodes’ virtual interfaces), storage usage, and firmware versions. It’s an easy way to get an overview and verify connectivity or version details ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=)).

**Syntax:** `tpi info`

There are no special sub-options for `info` (besides global flags like `--json` if you want JSON output).

**Output Details:** The info command typically returns a structured summary. Based on the BMC API, `info` includes:

- **IP Addresses:** It lists network interfaces on the BMC (excluding loopback). For each interface, you get the interface name, IP address, and MAC address ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=Returns%3A)). Usually, you’ll see one entry for the BMC’s Ethernet (e.g., `eth0` or `br0`). Example: `eth0: 192.168.1.50 (MAC 12:34:56:78:9A:BC)`.
- **Storage:** It shows total and free storage on the BMC’s internal flash and the microSD (if inserted) ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=Returns%3A)). For instance: `storage: internal_total=1GB, internal_free=200MB; microsd_total=16GB, microsd_free=15.5GB`. This helps to see if the BMC’s storage is nearly full or if an SD card is in use.
- Possibly **BMC Info:** In some outputs, it might also show the BMC firmware version or API version. (The BMC API has an `about` endpoint with `api`, `version`, `buildtime` ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=)), which might be included in `info` or shown separately with another command. If `info` doesn’t show firmware version, use `tpi --version` for the CLI version and check the BMC UI for firmware version.)
- **Node presence:** Not explicitly in `info`, but sometimes the BMC might list detected modules. The current `info` focuses on BMC aspects. To list what modules (CM4, RK1, etc.) are installed, there isn’t a direct CLI command – you infer by which nodes power on or perhaps check via IP assignments.

**Example:**

```bash
$ tpi info --json
```

Output (example in JSON format):

```json
{
  "ip": [
    {
      "iface": "eth0",
      "addr": "192.168.1.50",
      "mac": "12:34:56:78:9A:BC"
    }
  ],
  "storage": {
    "emmc_total": 1048576000,
    "emmc_free": 524288000,
    "sd_total": 0,
    "sd_free": 0
  }
}
``` 

In plain text (without `--json`), it might print something like:

```
Network Interfaces:
 - eth0: 192.168.1.50 (MAC 12:34:56:78:9A:BC)
Storage:
 - BMC Internal Flash: 1000 MB total, 500 MB free
 - microSD: not present
``` 

(The above is illustrative; actual formatting may vary.)

**Usage:**

- Run `tpi info` after first setting up to see if the BMC got an IP address from DHCP.
- Check `tpi info` before and after inserting an SD card in the BMC to confirm it’s recognized (it will show the SD’s free/total).
- It’s also useful to gather system info in scripts (using `--json` to parse easily).

**Notes:**

- **Authentication:** You need to be authenticated to get info (like other commands), since it queries the BMC. If not logged in, it will prompt or fail.
- **Comparison to UI:** The info here corresponds to what the BMC UI homepage shows (IP, MAC, storage). They should match ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=Returns%3A)).
- **No Node IPs:** This does *not* list the IPs of the compute nodes. The BMC doesn’t inherently know each node’s IP on the network (since that’s handled by your external DHCP and the nodes themselves). You’d have to log into each node or check your DHCP server for that. `tpi info` is about the management controller’s info.
- **MAC addresses for nodes:** All nodes’ Ethernet go through the on-board switch to the same external port(s), so from BMC’s perspective, there’s just one interface. It doesn’t enumerate node MACs here. To get node MACs, you’d need to boot them and check or possibly use the BMC API if it exposes a switch ARP table (not part of `info` as far as documented).
- **Troubleshooting:** If `tpi info` fails or times out, it indicates you might not be connected to the BMC properly (wrong host or network issue). If it returns but shows no IP (0.0.0.0 or blank), then the BMC might not have network config (e.g., if you haven’t plugged in Ethernet or your DHCP didn’t give an address).

Overall, `info` is a safe, read-only command to verify the BMC’s status and should always be one of the first things you run when diagnosing issues.

## `reboot` – Reboot the BMC (Baseboard Controller)

**Description:** The `reboot` command restarts the BMC itself. This is equivalent to rebooting the entire Turing Pi 2 board’s management system (like doing a power cycle of the control plane). **Warning:** When the BMC reboots, **all compute nodes will lose power** (because the power rails are managed by the BMC) ([Usage](https://docs.turingpi.com/docs/tpi-usage#:~:text=advanced%20%20Advanced%20node%20modes,will%20lose%20power%20until%20booted)). They will remain off until the BMC comes back up and you explicitly power the nodes on again. Use this command carefully, typically only when needed (for example, after a firmware update, or if the BMC software is malfunctioning and needs a restart).

**Syntax:** `tpi reboot`

No additional options (aside from global `--host`, `--user` if you need to specify how to connect/authenticate). It’s just a straightforward command.

**Behavior:**

- Upon invoking `tpi reboot`, the command will signal the BMC to reboot. The CLI may not give any special output other than perhaps “rebooting…” because it will soon be disconnected from the BMC.
- Your session (SSH or the `tpi` CLI connection) will drop as the BMC goes down. If you run it from a PC, you’ll likely get a connection error or timeout as the BMC shuts down mid-communication.
- The BMC will take a short time to reboot (usually under a minute). During this time, the BMC’s network interface will go down (so `tpi` can’t connect), and power to all nodes is cut (they will all shut off immediately as if power lost).
- Once the BMC boots up again, it will restore its network (DHCP etc.), and you can reconnect. The nodes by default do *not* auto-power-on after a BMC reboot (unless you have configured startup scripts as discussed in Automating section) – they stay off until you issue `tpi power on` for each or all ([Automating Node and USB Settings](https://docs.turingpi.com/docs/turing-pi2-automating-node-and-usb-settings#:~:text=chmod%20%2Bx%20%2Fetc%2Finit)).

**Use Cases:**

- **Apply Config Changes:** If you manually changed some BMC configuration (like network settings on BMC, or enabled a service), you might need to reboot for it to take effect.
- **Recover BMC State:** If the BMC web UI or API is behaving oddly, a reboot can clear issues (like any stuck USB or network states).
- **After Firmware Update:** Many firmware updates require a reboot. The `tpi firmware` command often triggers this automatically, but if not, you’d do `tpi reboot`.

**Example:**

```bash
$ tpi reboot
Rebooting BMC...
```

(You might see “Connection lost” because the BMC went down before responding.)

After maybe 30 seconds, you could try `tpi info` or ping the BMC to see if it’s back online. Then you would likely do `tpi power on` for each node to bring your cluster back up.

**Notes:**

- This is a drastic action: all running workloads on nodes will be abruptly powered off. Ensure that’s acceptable (for example, plan a maintenance window if using in a cluster).
- If you just need to **restart the BMC management service (bmcd)** without full reboot, there is an API call (`reload` in BMC API) ([BMC API](https://docs.turingpi.com/docs/turing-pi2-bmc-api#:~:text=)), but the CLI does not have a direct subcommand for that. `tpi reboot` is a full restart. Only use it if necessary.
- There is no prompt like “Are you sure?” – the command executes immediately. So double-check you meant to reboot the BMC and not, say, a single node (common confusion since `tpi power reset -n X` reboots a node, whereas `tpi reboot` reboots the BMC itself).
- If the BMC fails to come back (very rare), you might have to physically power-cycle the board or use the BMC’s serial console if available. Typically it comes back unless a firmware flash went wrong.
- While the BMC is rebooting, `tpi` commands will obviously not work. If you have an automated script, you might want it to wait and retry commands after a reboot.

---

## Additional Tips and Troubleshooting

- **Ensure Up-to-Date Software:** Both the BMC firmware and the `tpi` CLI on your PC should ideally be kept in sync (or at least both updated to latest versions) to avoid any version mismatches in API.
- **Command Help:** You can use `tpi <command> --help` for any command to get a quick summary of usage. For instance, `tpi power --help` will show the options and subcommands for power management.
- **Combining Commands:** The CLI does one action at a time. If you need to do a sequence (e.g., flash then power on, or change USB then power on), you must run them as separate commands or in a script. Order can matter (for example, when automating startup, set USB modes *before* powering on nodes as needed ([Automating Node and USB Settings](https://docs.turingpi.com/docs/turing-pi2-automating-node-and-usb-settings#:~:text=Important%20Considerations))).
- **Logging and Errors:** The `--json` flag can be useful to detect errors programmatically. In JSON mode, if an error occurs, it might return a JSON with an error message field. In normal mode, you’ll get a stderr output. If a command isn’t doing what you expect, try running with `--json` or `--verbose` (if supported) to see more detail.
- **Security:** After initial setup, change the default password (`turing`). The CLI will then need the new password for `--password` or when prompting. The token cache is stored locally on your PC (or BMC home directory if run there) – treat it as a credential. If you want to log out (invalidate token), you can remove the `tpi_token` file in the cache location mentioned earlier.
- **Non-interactive Scripting:** You can script `tpi` by providing `--user` and `--password` or having a token cached. Use `--json` for machine-readable output (e.g., to parse `tpi info` in a script that monitors the system). The CLI is designed to be automation-friendly.
- **Serial vs CLI vs UI:** You have multiple ways to manage the Turing Pi 2: the web UI, the CLI, and the direct serial console. They all ultimately control the same BMC functions. Some tasks are easier in one interface than another (for example, dragging an image file into the web UI might be easier for flashing if the CLI flash is limited; on the other hand, scripting 10 device reboots is easier with CLI). You can use them interchangeably, but avoid doing conflicting actions at the exact same time.

With this detailed documentation of `tpi` CLI commands, you should be able to perform full management of the Turing Pi 2 cluster programmatically or via command line, including power cycling nodes, configuring USB and network, updating firmware, flashing OS images, and accessing consoles. Each command provides a powerful interface to the BMC’s capabilities – combining them allows for automation of complex workflows (like bringing up a Kubernetes cluster across the nodes, etc.). Always test commands carefully on one node or a non-critical system to get familiar before automating against many nodes.
