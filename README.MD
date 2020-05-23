ether query service
=================
scan all ether transaction from geth

## Build

### 安装开发工具

    make, go, protoc

### 获取源代码

使用 https（帐号密码认证）

    export GIT_TERMINAL_PROMPT=1

使用 git（SSH key 认证）

    git config --global url."git@github.com:".insteadOf "https://github.com"

拉取代码并切到 dev 分支

    go get github.com/tokenbankteam/tb_server
    cd $GOPATH/src/github.com/tokenbankteam/tb_server
    git checkout dev

### 编译

    make


### 启动
```
nohup ./tb.etherquery.s --datadir /data --syncmode=full --gcmode=archive --cache=1024 --port 30302 --rpc --rpcaddr 172.24.55.19 --rpcport 7545 --ws --wsaddr 172.24.55.19 --wsport 7546 --wsorigins "*" --rpccorsdomain "*" --rpcvhosts "*" &

```

## CodeReview principle

### 1. 代码遵循基本分层，不能跨层访问

```
Controller逻辑层 ==> Service层 ==> Model层 ==> Database层
                              ==> Cache层
                ==> Config层  ==> Config层 ==> Config层

```

### 2. 数据结构不能跨层
```

1. 不要把协议层的数据结构传入Model层或者更底层
2. 不要把Controller层的gin.Context传到Service或者Model层

```

### 3. RPC定义Function返回值使用匿名

### 4. 命名风格，变量使用golang的统一风格（驼峰）， 文件名使用下划线分割

### 5. 所有的Function返回错误，必须显示处理

### 6. 所有代码必须格式化
