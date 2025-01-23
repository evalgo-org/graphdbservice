from operator import add
from dask.distributed import Client

client = Client('172.20.0.2:8786')
x = client.submit(add, 1, 2)
result = client.gather(x)
print(result)
