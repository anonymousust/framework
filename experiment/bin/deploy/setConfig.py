import json
import sys

# 检查命令行参数的长度
if len(sys.argv) < 3:
    print("Usage: python setConfig.py <ByzNo>")
    sys.exit(1)

# 从命令行参数获取 ByzNo 的值
byz_no = sys.argv[1]
seed = sys.argv[2]

try:
    with open('config.json', 'r') as json_file:
        data = json.load(json_file)
except FileNotFoundError:
    data = {}
# 将 ByzNo 的值设置为从命令行获取的值
data["byzNo"] = int(byz_no)  
data["seed"] = int(seed)  
with open('config.json', 'w') as json_file:
    json.dump(data, json_file, indent=4)