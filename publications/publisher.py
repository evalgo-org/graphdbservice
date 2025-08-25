import asyncio
from os import environ
from dotenv import load_dotenv
import json

from faststream import FastStream
from faststream.rabbit import RabbitQueue, RabbitBroker, RabbitMessage

load_dotenv()

async def main():
    try:
        msg = {   "version": "v1",
            "kaeser-publications": [
                    {
                            "selectionTemplate": "https://data.kaeser.com/KKH/SelectionTemplateEG",
                            "language_tag": [
                                    "en-GB",
                                    "de-DE"
                            ],
                            "include_prices": "true",
                            "include_characteristics": "true",
                            "include_options": "true",
                            "include_associatedArticles": "true",
                            "salesOrg": "0002",
                            "region": "EG",
                            "country": "DE",
                            "repoConsumption": "TestAndrei",
                            "repoTransformation": "TestAndreiTransformation"
                    }
            ]
        }
        # msg = {
        #     "version":"v1",
        #     "kaeser-publications": [
        #         {
        #             "selectionTemplate": "https://data.kaeser.com/KKH/SelectionTemplateEG",
        #             "language_tag": ["en-GB", "de-DE"],
        #             "include_prices": "true",
        #             "include_characteristics": "true",
        #             "include_options": "true",
        #             "include_associatedArticles": "true",
        #             "salesOrg": "0002",
        #             "region": "EG",
        #             "country": "DE",
        #             "repoConsumption": "Consumption-Navigator-DE-Intern-All",
        #             "repoTransformation": "Transformation-Navigator-DE-Intern",
        #         }
        #     ]
        # }
        queue_out = RabbitQueue(environ.get("RABBITMQ_QUEUE"))
        broker_uri = environ.get("RABBITMQ_CONN_URL")
        async with RabbitBroker(broker_uri,max_consumers=int(environ.get("RABBITMQ_MAX_CONSUMERS"))) as broker:
            await broker.declare_queue(queue_out)
            await broker.publish(
                message=json.dumps(msg),
                queue=queue_out,
            )
        await broker.close()
    except Exception as e:
        print("error: " + str(e))

if __name__ == "__main__":
    asyncio.run(main())
