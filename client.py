import asyncio
import simple_websocket
import websockets
from simple_websocket import AioClient, ConnectionClosed
import random


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

async def main():
    async for ws in websockets.connect('ws://localhost:80/ws'):
   #ws = await AioClient.connect('ws://0.tcp.ap.ngrok.io:12476/ws') - use this address when running remotely on IOT client and update URI

        try:
            print("Connecting to server")
            await ws.send ({"command":"register","company_id":"1","charger_id":"12135352","status":1})
            
            # rest of the following logic is for arbitrary message testing only, not real business logic
            await asyncio.sleep(5)
            await ws.send("start")

            await asyncio.sleep(5)
            await ws.send("stop")

            while True:
                #data = input('> ')
                
                #rnum = round(10*random.random())
                #await ws.send(str(rnum))

                data = await ws.recv()
                print(f'< {data}')

                await handle_message(data, ws)
                print("client has handled the message")

                #await asyncio.sleep(2)

        except (KeyboardInterrupt, EOFError):
            print("keyboard interrupt or EOFError")
            await ws.close()
        
        except (ConnectionClosed):
            print("Connection closed")
            continue

        except websockets.exceptions.ConnectionClosedError:
            print("Connection closed error")
            # wait a while for random period before reconnecting, to avoid the charge points overloading server with reconnection requests
            await asyncio.sleep(rnum)
            print("Trying to connect...")   
            continue


if __name__ == '__main__':
    asyncio.run(main())