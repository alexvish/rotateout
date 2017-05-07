Rotateout
=========

rotateout is a simple utility that writes piped output of a command to a rotated file

## Usage:
```
   <command>|rotateout [OPTIONS] [logBaseName]
```
### Options:
```
  -f, --format=      stftime format of time suffix of rotated file: 
                     Example: %Y-%m-%d-%H_%M_%S

      --utc          If set utc time will be used to format rotation timestamps

  -e, --ext=         Log file extension (default: .log)
```
### Rotation Options:
```
  -t, --time=        Duration between log rotaitons. Value is a sequence of decimal numbers with optional fraction 
                     followed by unit. Default unnit is s (second) Walid units are "w", "d", "h", "m", "s". 
                     Example: 3d12.5h 
                     (default: 1w)
                     
  -s, --size=        Size of current file that triggers log rotations. Value is a sequence of decimal numbers with 
                     optional fraction followed by unit. Default unnit is b (byte) Walid units are "g", "m", "k", "b". 
                     Example: 2m512.15k100b 
                     (default: 1g)
```
## Example:
```sh
# writes to ./logs/out.log, rotates to ./logs/out.<num>.log
cmdnout | rotateout logs/out
# rotate on size 1m or every 3 hours (what occur earlier) to logs with time pattern
# Example: logs/out.2017-05-08-01_01_32.log
cmdnout | rotateout -s 1m -t 3h -f ' %Y-%m-%d-%H_%M_%S' logs/out
```
## License
GPL V2
