
# UART

With `tpi` it's possible to read and write data to/from nodes over serial, effectively emulating a (rather inefficient) TTY session. However, there are two things to be aware of:

- UART is disabled by default in Raspberry Pi OS,
- The non-interactive usage with the `tpi` command can be performed interactively with `picocom` which comes pre-installed on the BMC.

## Enabling UART in Raspberry Pi OS

Serial functionality is controlled in Raspberry Pi OS by a value `enable_uart` in OS configuration file called `/boot/config.txt`. There are several ways to toggle it:

- Using raspi-config:
    - After you log in to your node, execute `sudo raspi-config`;
    - Navigate to "3 Interface Options";
    - Enable "I6 Serial Port".
- Manually:
    - After you log in, edit the config file `sudo vi /boot/config.txt`;
    - Add `enable_uart=1` under `[all]`.
- Without logging in:
    - Mount the device's boot partition in your BMC (see [Accessing nodes's filesystems](https://docs.turingpi.com/docs/accessing-nodes-filesystems))
    - Edit `config.txt` and add `enable_uart=1` under `[all]`.
    - Unmount partition.

Changes take effect after restart.

## Using `picocom`

- Log in to your BMC, e.g. `ssh root@turingpi.local`    
- Determine the serial pseudo-device of the node
    |Node #|BMC Firmware V2.0.5 and Older|BMD Firmware V2.1.0 and Newer|
    |---|---|---|
    |1|`/dev/ttyS2`|`/dev/ttyS1`|
    |2|`/dev/ttyS1`|`/dev/ttyS2`|
    |3|`/dev/ttyS4`|`/dev/ttyS3`|
    |4|`/dev/ttyS5`|`/dev/ttyS4`|
    
- Execute command with an appropriate device path:  
    `picocom /dev/ttyS2 -b 115200`
- Turn the node on and after several seconds you will see the Linux boot log. You may now log in and execute any commands of your choice.
    

## Using `tpi`

Reading and writing over serial can be accomplished with commands `tpi uart get` and `tpi uart set` respectively. The former performs reading of stored data:

```
$ tpi uart --node 1 get
[    0.008458] CPU1: Booted secondary processor 0x0000000001 [0x410fd083]
[..]
[    5.610444] systemd[1]: Started Journal Service.
Debian GNU/Linux 11 raspberrypi ttyS0
raspberrypi login:
```

And the latter writes data, effectively executing a command:


```
$ tpi uart --node 1 set --cmd 'username'
$ tpi uart --node 1 set --cmd 'password'
$ tpi uart --node 1 get
[..]
Linux raspberrypi 5.15.84-v8+ #1613 SMP PREEMPT [..]
$ tpi uart --node 1 set --cmd 'echo hi'
$ tpi uart --node 1 get
echo hi
hi
```



