import json
import asyncio
import openziti
from typing import Any
from os import environ
from dotenv import load_dotenv
from pathlib import Path

from faststream import FastStream
from faststream.rabbit import RabbitExchange, RabbitQueue, RabbitBroker, RabbitMessage, ExchangeType

from scalablePublication import publication_flow

if Path(".env").is_file():
    load_dotenv()

enable_ziti = False
if Path('/app/publications.consumer.json').is_file():
    ztx = openziti.load('/app/publications.consumer.json')
    enable_ziti = True

queue = environ.get("RABBITMQ_QUEUE")
broker_uri = environ.get("RABBITMQ_CONN_URL")
broker = RabbitBroker(broker_uri,max_consumers=int(environ.get("RABBITMQ_MAX_CONSUMERS")))
app = FastStream(broker)

def run_publications(publications):
    # run the publications sequentialy
    for pub in publications:
        # enforce the target to the configuration because it is defined in the queue name
        pub["targetGraphDB"]=environ.get("TARGET_GRAPH_INSTANCE")
        # enforce the server the computation should take place
        pub["server"]=environ.get("COMPUTE_GRAPH_SERVER")
        publication_flow([pub])

@broker.subscriber(queue=queue, no_ack=False)
async def pdf_copy_couchdb(body: Any, msg: RabbitMessage):
    mmsg = None
    if type(body) == dict:
        mmsg = body
    elif type(body) == str:
        mmsg = json.loads(body)
    print(mmsg)
    if not "version" in mmsg:
       # can not work on the message required field version missing
       print("escalation can not work on the message required field version missing")
       return
    if mmsg["version"] != "v1":
       print("can not work on the message unsupported version: " + mmsg["version"])
       return
    if not "kaeser-publications" in mmsg:
        # can not work on the message required field kaeser-publications missing
       print("can not work on the message required field kaeser-publications missing")
       return
    else:
        if enable_ziti:
            with openziti.monkeypatch():
                run_publications(mmsg["kaeser-publications"])
        else:
            run_publications(mmsg["kaeser-publications"])
    await msg.ack()

async def main():
    await broker.connect()
    await broker.declare_queue(RabbitQueue(queue))
    await app.run()

if __name__ == "__main__":
    asyncio.run(main())
