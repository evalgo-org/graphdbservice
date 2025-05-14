from operator import add
import time
import s3fs

def added(x, y):
    time.sleep(5)
    print(add(x,y))

def test_dask_flow():
    futures = []
    for i in range(0,100):
        futures.append(added.submit(i, 5))
    print(futures)
    list(map(lambda f: print(f.result()), futures))

if __name__ == '__main__':
    test_dask_flow()
