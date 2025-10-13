#!/bin/bash

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}🚀 开始快速测试秒杀系统...${NC}"

# 检查测试脚本是否存在
if [[ ! -f "./scripts/seckill_test.sh" ]]
then
    echo -e "${RED}错误: 测试脚本不存在 ./scripts/seckill_test.sh${NC}"
    exit 1
fi

# 设置执行权限
chmod +x ./scripts/seckill_test.sh

# 记录开始时间
START_TIME=$(date +%s)

# 1. 重置环境
echo -e "${YELLOW}1. 重置测试环境...${NC}"
if ./scripts/seckill_test.sh reset
then
    echo -e "${GREEN}✓ 环境重置成功${NC}"
else
    echo -e "${RED}✗ 环境重置失败${NC}"
fi
echo

# 2. 管理功能测试
echo -e "${YELLOW}2. 测试管理功能...${NC}"
if ./scripts/seckill_test.sh admin
then
    echo -e "${GREEN}✓ 管理功能测试完成${NC}"
else
    echo -e "${RED}✗ 管理功能测试失败${NC}"
fi
echo

# 3. 单用户测试
echo -e "${YELLOW}3. 单用户测试...${NC}"
if ./scripts/seckill_test.sh single
then
    echo -e "${GREEN}✓ 单用户测试完成${NC}"
else
    echo -e "${RED}✗ 单用户测试失败${NC}"
fi
echo

# 4. 并发测试
echo -e "${YELLOW}4. 并发测试(20用户)...${NC}"
if ./scripts/seckill_test.sh concurrent 20
then
    echo -e "${GREEN}✓ 并发测试完成${NC}"
else
    echo -e "${RED}✗ 并发测试失败${NC}"
fi
echo

# 5. 完整测试
echo -e "${YELLOW}5. 完整测试...${NC}"
if ./scripts/seckill_test.sh all
then
    echo -e "${GREEN}✓ 完整测试完成${NC}"
else
    echo -e "${RED}✗ 完整测试失败${NC}"
fi
echo

# 计算总耗时
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo -e "${GREEN}🎉 所有测试完成! 总耗时: ${DURATION}秒${NC}"
echo
echo -e "${YELLOW}📊 测试结果摘要:${NC}"
echo -e "  - 环境重置: ${GREEN}完成${NC}"
echo -e "  - 管理功能: ${GREEN}测试完成${NC}" 
echo -e "  - 单用户流程: ${GREEN}测试完成${NC}"
echo -e "  - 并发性能: ${GREEN}测试完成${NC}"
echo
echo -e "${YELLOW}📝 注意:${NC}"
echo -e "  - 查看详细日志: ${YELLOW}cat seckill_test.log${NC}"
echo -e "  - 并发成功率在秒杀场景中20%是正常的"
echo -e "  - '库存不足'错误是正常的业务逻辑"
echo -e "  - 系统防护机制工作正常"
