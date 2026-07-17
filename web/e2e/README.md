# E2E 测试

启动测试方式是先手动启动数据库，这里配置了 docker 方便启动：
 `cd` 到 docker 下，输入命令 `docker compose up -d` 启动，如果需要关闭并删除数据库数据，可以使用命令 `docker compose down -v`。

启动好数据库后 `cd` 到 web 目录下，输入 `pnpm e2e` 即可启动 e2e 测试。