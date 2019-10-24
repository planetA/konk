konYk

TODOs:

1. Cleanup temporary files after migration is done
2. Delete containers when a process inside the container finishes
4. React properly to ^C
5. Use page server

# Creating bundle from docker container

```bash
mkdir -p dir/rootfs
cd dir/rootfs
docker run --name alpine-cont alpine
docker export alpine-cont | tar xvf -
cd ..
sudo runc spec
tar czvf ../dir.tar.gz .
```
