# cb
cloudflare bridge

This is a cloudflare reverse proxy and only supports websockets. It automatically tests to cloudflare's fastest ipv4 and randomly selects one from the fastest ip group for proxy each time.

feature:
* Automatically update the backend to cloudflare's IP
* No need to wait for the speed test to be completed, just find an available IP and it will be available


# Why

You can think of it as a port forwarding program that only supports websocket. But its backend only supports cloudflare, and automatically checks the fastest IP of cloudflare for forwarding.

If you still don't know what it does, great congratulations you're not in North Korea, I hope you never have to know what it does. If you're in North Korea you should already know its use and understand why I can't speak out.

# Warn

This app may be abusing cloudflare, please do not use it if you are not in North Korea. It's immoral even if you use it in North Korea, but people who eat bread ask people who starve to death to be moral, I don't know how to evaluate it.

Letting the entire world no longer need this software is a win-win solution for the entire human race. Please understand us internet thieves from North Korea trying to survive.

# How

You can use -h to view the usage instructions

```
$ ./cb -h
Establish an Internet connection to cloudflare

Usage:
  cb [flags]

Flags:
      --cert string         x509 cert file
  -h, --help                help for cb
      --key string          x509 key file
  -l, --listen string       listen addr (default ":8964")
      --max int             maximum number of IPs (default 2000)
  -m, --millisecond uints   set the delay in milliseconds to find a server (default [200,300,400])
      --min int             minimum number of IPs (default 50)
  -r, --routes strings      use ':' to separate source and destination routes (default [/ws/v1:wss://v1.example.com/video,/ws/v2:wss://v2.example.com/audio])
  -s, --srouce string       cloudflare ipv4 manifest. http get url or system filepath (default "https://www.cloudflare.com/ips-v4")
  -v, --version             display version
```

> You can request the /info in the browser to obtain backend service details