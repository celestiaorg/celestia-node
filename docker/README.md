# Dockerfile for `celestia-node`

To build on a M1 Mac:
```
docker build --ssh default --platform linux/x86_64 .
```

To run on a M1 Mac:
```
docker run -it --platform linux/x86_64 <image sha>
```

If building/running on a x86_64 linux machine you should be able to leave off the `--platform` flag, but this hasn't been tested.