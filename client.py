import asyncio
import simple_websocket
import websockets
from simple_websocket import AioClient, ConnectionClosed
import random
import json
import ssl
import pathlib
import threading
import time

done = False
status = "pending"
chargerId = "59"

#define message handling logic
async def handle_message(message, ws):
    if(message=="start"):
        await ws.send("Start charging")
        
        #action to do when receive "start" message
        print("Start charger, Switch on LED")

    elif(message=="stop"):
        await ws.send("Stop charging")
        
        #action to do when receive "stop" message
        print("Stop charger, Switch off LED")

    else:
        # do nothing when there are other messages
        #await ws.send(message)
        print("no amendments to msg")

rnum = round(10*random.random())
# ssl_context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
# localhost_pem = pathlib.Path(__file__).with_name("key.pub")
# ssl_context.load_cert_chain(localhost_pem)

async def keepalive(ws):
    print("launch keepalive")
    while(True):
        print("send keepalive")
        event = {"command":"keepalive","charger_id":chargerId, "status": status}
        print("sending keepalive")
        await ws.send(json.dumps(event))
        await asyncio.sleep(5)


async def main():
    print("launching....")
    global status

    async with websockets.connect('ws://localhost:9090/api/v1/ws') as ws:
    # async for ws in websockets.connect('wss://localhost:443/api/v1/ws', ssl=ssl_context):
   #ws = await AioClient.connect('ws://0.tcp.ap.ngrok.io:12476/ws') - use this address when running remotely on IOT client and update URI
        x = asyncio.create_task(keepalive(ws))

        try:
            print("Connecting to server")
            status = "available"
            event = {"command":"register","company_id":"1","charger_id":chargerId,"status":status}
            await ws.send(json.dumps(event))
            

            # rest of the following logic is for arbitrary message testing only, not real business logic
            await asyncio.sleep(5)
            event["command"] = "starting"
            event["status"] = status
            await ws.send(json.dumps(event))

            await asyncio.sleep(5)
            event["command"] = "running"
            event["status"] = "available"
            status = "available"
            await ws.send(json.dumps(event))


            while True:
                #data = input('> ')
                
                #rnum = round(10*random.random())
                #await ws.send(str(rnum))

                data = await ws.recv()
                print(f'< {data}')
                
                obj = json.loads(data)
                if obj["status"] != None:
                    status = obj["status"]

                # await handle_message(data, ws)
                # print("client has handled the message")

                #await asyncio.sleep(2)

        except (asyncio.CancelledError):
            print("keyboard interrupt or EOFError")      
            event = {"command":"unregister","company_id":"1","charger_id":chargerId,"status":"available"}
            await ws.send (json.dumps(event))
            data = await ws.recv()
            print(f'< {data}')
            await ws.close()
            exit(0)

        except (KeyboardInterrupt, EOFError):
            print("keyboard interrupt or EOFError")
            await ws.close()
        
        except (ConnectionClosed):
            print("Connection closed")

        except websockets.exceptions.ConnectionClosedError:
            print("Connection closed error")
            # wait a while for random period before reconnecting, to avoid the charge points overloading server with reconnection requests
            await asyncio.sleep(rnum)
            print("Trying to connect...")   
    
    print("launching keepalive")
    # x = asyncio.create_task(keepalive(ws))



if __name__ == '__main__':
    asyncio.run(main())