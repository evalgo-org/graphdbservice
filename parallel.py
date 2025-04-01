from operator import add
from prefect_dask import DaskTaskRunner, get_dask_client
from prefect import flow, task
import time

@task(log_prints=True)
def added(x, y):
    time.sleep(5)
    print(add(x,y))

@flow(task_runner=DaskTaskRunner(address="172.20.0.3:8786"), log_prints=True)
def test_dask_flow():
    futures = []
    for i in range(0,100):
        futures.append(added.submit(i, 5))
    print(futures)
    list(map(lambda f: print(f.result()), futures))

if __name__ == '__main__':
    test_dask_flow()
