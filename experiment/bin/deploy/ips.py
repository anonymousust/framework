import json

# 基础 IP 地址和端口，用于往config里写节点地址
base_ips = [
'8.149.247.237',
'8.139.255.195',
'8.149.135.103',
'8.149.132.171',

]
base_port = 3735

# 存储节点信息的字典
nodes = {}

for i in range(1, 61):
    ip = base_ips[(i - 1) % len(base_ips)]
    port = base_port + (i - 1) // len(base_ips)
    nodes[str(i)] = f"tcp://{ip}:{port}"

try:
    with open('config.json', 'r') as json_file:
        data = json.load(json_file)
except FileNotFoundError:
    data = {}

data["address"] = nodes

with open('config.json', 'w') as json_file:
    json.dump(data, json_file, indent=4)
# 存储节点信息的字典
nodes = {}
base_port = 8070

for i in range(1, 61):
    ip = base_ips[(i - 1) % len(base_ips)]
    port = base_port + (i - 1) // len(base_ips)
    nodes[str(i)] = f"http://{ip}:{port}"

# 将节点信息转换为 JSON 格式
json_data = json.dumps(nodes, indent=4)

# 输出 JSON 数据
try:
    with open('config.json', 'r') as json_file:
        data = json.load(json_file)
except FileNotFoundError:
    data = {}

data['http_address'] = nodes
# 将 JSON 数据写入文件
with open('config.json', 'w') as json_file:
    json.dump(data, json_file, indent=4)