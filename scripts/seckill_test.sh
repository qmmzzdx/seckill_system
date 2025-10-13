#!/bin/bash

# 秒杀系统完整测试脚本 - 优化版
# 基于当前路由配置

# 配置
BASE_URL="http://localhost:8000/api"
GOODS_ID=1001
ADMIN_PARAM="admin=1"
LOG_FILE="seckill_test.log"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 测试统计
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# 初始化日志文件
> "$LOG_FILE"

# 日志函数
function log_info()
{
    local msg="$1"
    echo -e "${BLUE}[INFO]${NC} $msg" | tee -a "$LOG_FILE" >/dev/tty
}

function log_success()
{
    local msg="$1"
    echo -e "${GREEN}[SUCCESS]${NC} $msg" | tee -a "$LOG_FILE" >/dev/tty
    ((PASSED_TESTS++))
    ((TOTAL_TESTS++))
}

function log_warning()
{
    local msg="$1"
    echo -e "${YELLOW}[WARNING]${NC} $msg" | tee -a "$LOG_FILE" >/dev/tty
}

function log_error()
{
    local msg="$1"
    echo -e "${RED}[ERROR]${NC} $msg" | tee -a "$LOG_FILE" >/dev/tty
    ((FAILED_TESTS++))
    ((TOTAL_TESTS++))
}

function log_debug()
{
    local msg="$1"
    echo "[DEBUG] $msg" >> "$LOG_FILE"
}

function log_step()
{
    local msg="$1"
    echo -e "${PURPLE}[STEP]${NC} $msg" | tee -a "$LOG_FILE" >/dev/tty
}

function log_result()
{
    local test_name="$1"
    local status="$2"
    local message="$3"
    
    if [[ "$status" == "success" ]]
    then
        echo -e "${GREEN}✓${NC} $test_name: $message" | tee -a "$LOG_FILE" >/dev/tty
        ((PASSED_TESTS++))
    else
        echo -e "${RED}✗${NC} $test_name: $message" | tee -a "$LOG_FILE" >/dev/tty
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))
}

# 打印测试统计
function print_test_stats()
{
    echo
    echo -e "${CYAN}=== 测试统计 ===${NC}"
    echo -e "总测试数: $TOTAL_TESTS"
    echo -e "${GREEN}通过: $PASSED_TESTS${NC}"
    echo -e "${RED}失败: $FAILED_TESTS${NC}"
    
    if [[ $TOTAL_TESTS -gt 0 ]]
    then
        local success_rate=$(echo "scale=2; $PASSED_TESTS * 100 / $TOTAL_TESTS" | bc)
        echo -e "成功率: ${success_rate}%"
    fi
    
    if [[ $FAILED_TESTS -eq 0 ]]
    then
        echo -e "${GREEN}🎉 所有测试通过！${NC}"
    else
        echo -e "${YELLOW}⚠️  有 $FAILED_TESTS 个测试失败${NC}"
    fi
    echo
}

# 检查服务状态
function check_service()
{
    log_step "检查服务状态..."
    if curl -s --connect-timeout 5 "$BASE_URL/goods/$GOODS_ID" > /dev/null
    then
        log_success "服务正常运行"
        return 0
    else
        log_error "服务不可达，请确保服务正在运行"
        return 1
    fi
}

# 重置测试环境
function reset_environment()
{
    log_step "重置测试环境..."
    
    local reset_ok=true
    
    # 重置数据库
    log_info "重置数据库..."
    RESPONSE=$(curl -s -X POST "$BASE_URL/admin/reset_db?$ADMIN_PARAM&goods_id=$GOODS_ID")
    log_debug "重置数据库响应: $RESPONSE"
    if echo "$RESPONSE" | grep -q '"code":0'
    then
        log_success "数据库重置成功"
    else
        log_error "数据库重置失败: $RESPONSE"
        reset_ok=false
    fi
    
    # 等待数据库重置完成
    sleep 2
    
    # 预加载库存
    log_info "预加载库存..."
    RESPONSE=$(curl -s -X POST "$BASE_URL/admin/preload/$GOODS_ID?$ADMIN_PARAM")
    log_debug "预加载库存响应: $RESPONSE"
    if echo "$RESPONSE" | grep -q '"code":0'
    then
        log_success "库存预加载成功"
    else
        log_error "库存预加载失败: $RESPONSE"
        reset_ok=false
    fi
    
    # 设置秒杀开启
    log_info "开启秒杀活动..."
    RESPONSE=$(curl -s -X POST "$BASE_URL/admin/config/seckill/enable?$ADMIN_PARAM&enabled=true")
    log_debug "开启秒杀活动响应: $RESPONSE"
    if echo "$RESPONSE" | grep -q '"code":0'
    then
        log_success "秒杀活动已开启"
    else
        log_error "开启秒杀活动失败: $RESPONSE"
        reset_ok=false
    fi
    
    # 设置高限流
    log_info "设置限流..."
    RESPONSE=$(curl -s -X POST "$BASE_URL/admin/config/rate_limit?$ADMIN_PARAM&limit=1000")
    log_debug "设置限流响应: $RESPONSE"
    if echo "$RESPONSE" | grep -q '"code":0'
    then
        log_success "限流设置成功"
    else
        log_warning "限流设置失败: $RESPONSE"
    fi
    
    # 等待配置生效
    sleep 1
    
    if $reset_ok; then
        log_success "测试环境重置完成"
    else
        log_warning "测试环境重置部分失败，但继续测试..."
    fi
    return 0
}

# 生成用户token
function generate_user_token()
{
    local user_id=$1
    local silent=${2:-false}
    
    if ! $silent
    then
        log_info "为用户 $user_id 生成token..."
    fi
    
    RESPONSE=$(curl -s -X GET "$BASE_URL/auth/create_user_token?user_id=$user_id")
    log_debug "生成token原始响应: $RESPONSE"
    TOKEN=$(echo "$RESPONSE" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
    
    if [[ -n "$TOKEN" ]]
    then
        if ! $silent
        then
            log_success "用户 $user_id token生成成功"
        fi
        log_debug "生成的token: $TOKEN"
        printf "%s" "$TOKEN"
        return 0
    else
        if ! $silent
        then
            log_error "生成token失败: $RESPONSE"
        fi
        return 1
    fi
}

# 验证用户token
function verify_user_token()
{
    local token=$1
    
    log_info "验证用户token..."
    
    RESPONSE=$(curl -s -X GET "$BASE_URL/auth/verify_user_token?token=$token")
    log_debug "验证token响应: $RESPONSE"
    
    if echo "$RESPONSE" | grep -q '"code":0' && echo "$RESPONSE" | grep -q '"valid":true'
    then
        log_success "Token验证成功"
        return 0
    else
        log_error "Token验证失败: $RESPONSE"
        return 1
    fi
}

# 获取秒杀令牌
function get_seckill_token()
{
    local user_token=$1
    local goods_id=$2
    local silent=${3:-false}
    
    if ! $silent
    then
        log_info "获取秒杀令牌 (商品ID: $goods_id)"
    fi
    
    RESPONSE=$(curl -s -w "HTTP_STATUS:%{http_code}" -X POST "$BASE_URL/seckill/token?gid=$goods_id" -H "Authorization: $user_token")
    
    HTTP_STATUS=$(echo "$RESPONSE" | grep -o "HTTP_STATUS:[0-9]*" | cut -d: -f2)
    RESPONSE_BODY=$(echo "$RESPONSE" | sed 's/HTTP_STATUS:[0-9]*//')
    
    log_debug "获取秒杀令牌HTTP状态码: $HTTP_STATUS"
    log_debug "获取秒杀令牌响应体: $RESPONSE_BODY"
    
    if [[ "$HTTP_STATUS" -eq 200 ]] && echo "$RESPONSE_BODY" | grep -q '"code":0'
    then
        SECKILL_TOKEN=$(echo "$RESPONSE_BODY" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
        if [[ -n "$SECKILL_TOKEN" ]]
        then
            if ! $silent
            then
                log_success "获取秒杀令牌成功"
            fi
            log_debug "秒杀令牌: $SECKILL_TOKEN"
            printf "%s" "$SECKILL_TOKEN"
            return 0
        fi
    fi
    
    if ! $silent
    then
        ERROR_MSG=$(echo "$RESPONSE_BODY" | grep -o '"error":"[^"]*' | cut -d'"' -f4)
        if [[ -z "$ERROR_MSG" ]]
        then
            ERROR_MSG="HTTP状态码: $HTTP_STATUS"
        fi
        log_error "获取秒杀令牌失败: $ERROR_MSG"
    fi
    return 1
}

# 执行秒杀
function execute_seckill()
{
    local user_token=$1
    local seckill_token=$2
    local goods_id=$3
    local silent=${4:-false}
    
    if ! $silent
    then
        log_info "执行秒杀 (商品ID: $goods_id)"
    fi
    
    RESPONSE=$(curl -s -w "HTTP_STATUS:%{http_code}" -X POST "$BASE_URL/seckill?gid=$goods_id&token=$seckill_token" -H "Authorization: $user_token")
    
    HTTP_STATUS=$(echo "$RESPONSE" | grep -o "HTTP_STATUS:[0-9]*" | cut -d: -f2)
    RESPONSE_BODY=$(echo "$RESPONSE" | sed 's/HTTP_STATUS:[0-9]*//')
    
    log_debug "执行秒杀HTTP状态码: $HTTP_STATUS"
    log_debug "执行秒杀响应体: $RESPONSE_BODY"
    
    if [ "$HTTP_STATUS" -eq 200 ] && echo "$RESPONSE_BODY" | grep -q '"code":0'
    then
        ORDER_ID=$(echo "$RESPONSE_BODY" | grep -o '"order_id":"[^"]*' | cut -d'"' -f4)
        if [[ -n "$ORDER_ID" ]]
        then
            if ! $silent
            then
                log_success "秒杀成功"
            fi
            log_debug "订单ID: $ORDER_ID"
            printf "%s" "$ORDER_ID"
            return 0
        fi
    fi
    
    if ! $silent
    then
        ERROR_MSG=$(echo "$RESPONSE_BODY" | grep -o '"error":"[^"]*' | cut -d'"' -f4)
        if [[ -z "$ERROR_MSG" ]]
        then
            ERROR_MSG="HTTP状态码: $HTTP_STATUS"
        fi
        log_error "秒杀失败: $ERROR_MSG"
    fi
    return 1
}

# 模拟支付
function simulate_payment()
{
    local user_token=$1
    local order_id=$2
    local success=$3
    
    log_info "模拟支付: 订单 $order_id, 成功: $success"
    
    RESPONSE=$(curl -s -X POST "$BASE_URL/payment/simulate?order_id=$order_id&success=$success" -H "Authorization: $user_token")
    log_debug "模拟支付响应: $RESPONSE"
    
    if echo "$RESPONSE" | grep -q '"code":0'
    then
        log_success "支付模拟成功"
        return 0
    else
        log_error "支付模拟失败: $RESPONSE"
        return 1
    fi
}

# 获取商品信息
function get_goods_info()
{
    local goods_id=$1
    log_info "获取商品 $goods_id 信息..."
    
    RESPONSE=$(curl -s -X GET "$BASE_URL/goods/$goods_id")
    log_debug "商品信息响应: $RESPONSE"
    echo "$RESPONSE"
}

# 单用户完整流程测试
function single_user_test()
{
    local user_id=$1
    local goods_id=$2
    
    log_step "=== 开始单用户测试 (用户ID: $user_id, 商品ID: $goods_id) ==="
    
    local test_ok=true
    
    # 生成用户token
    log_info "步骤1: 生成用户token"
    USER_TOKEN=$(generate_user_token $user_id)
    if [[ -z "$USER_TOKEN" ]]
    then
        log_error "用户token生成失败，跳过该用户测试"
        return 1
    fi
    
    # 验证token
    log_info "步骤2: 验证用户token"
    if ! verify_user_token "$USER_TOKEN"
    then
        log_error "用户token验证失败，跳过该用户测试"
        return 1
    fi
    
    # 获取秒杀令牌
    log_info "步骤3: 获取秒杀令牌"
    SECKILL_TOKEN=$(get_seckill_token "$USER_TOKEN" $goods_id)
    if [[ -z "$SECKILL_TOKEN" ]]
    then
        log_error "获取秒杀令牌失败，跳过该用户测试"
        return 1
    fi
    
    # 执行秒杀
    log_info "步骤4: 执行秒杀"
    ORDER_ID=$(execute_seckill "$USER_TOKEN" "$SECKILL_TOKEN" $goods_id)
    if [[ -n "$ORDER_ID" ]]
    then
        # 模拟支付
        log_info "步骤5: 模拟支付"
        if simulate_payment "$USER_TOKEN" "$ORDER_ID" "true"
        then
            log_success "单用户测试完成 - 订单ID: $ORDER_ID"
            log_result "单用户测试" "success" "用户 $user_id 成功完成秒杀流程"
        else
            log_result "单用户测试" "failed" "支付模拟失败"
            test_ok=false
        fi
    else
        log_result "单用户测试" "failed" "秒杀执行失败"
        test_ok=false
    fi
    
    log_info "=== 单用户测试完成 ==="
    return $([ "$test_ok" = true ] && echo 0 || echo 1)
}

# 并发测试
function concurrent_test()
{
    local concurrent_users=$1
    local goods_id=$2
    
    log_step "=== 开始并发测试 (并发数: $concurrent_users, 商品ID: $goods_id) ==="
    
    SUCCESS_COUNT=0
    FAIL_COUNT=0
    
    # 创建临时文件记录结果
    RESULT_FILE=$(mktemp)
    ERROR_FILE=$(mktemp)
    
    # 进度显示
    echo -n "进度: "
    
    for i in $(seq 1 $concurrent_users)
    do
        user_id=$((1000 + i))
        (
            # 生成用户token
            USER_TOKEN=$(generate_user_token $user_id true 2>> "$ERROR_FILE")
            if [[ -z "$USER_TOKEN" ]]
            then
                echo "FAIL:用户 $user_id 生成token失败" >> "$RESULT_FILE"
                exit 1
            fi
            
            # 获取秒杀令牌
            SECKILL_TOKEN=$(get_seckill_token "$USER_TOKEN" $goods_id true 2>> "$ERROR_FILE")
            if [[ -z "$SECKILL_TOKEN" ]]
            then
                echo "FAIL:用户 $user_id 获取秒杀令牌失败" >> "$RESULT_FILE"
                exit 1
            fi
            
            # 执行秒杀
            ORDER_ID=$(execute_seckill "$USER_TOKEN" "$SECKILL_TOKEN" $goods_id true 2>> "$ERROR_FILE")
            if [[ -n "$ORDER_ID" ]]
            then
                echo "SUCCESS:用户 $user_id 秒杀成功" >> "$RESULT_FILE"
            else
                echo "FAIL:用户 $user_id 秒杀失败" >> "$RESULT_FILE"
            fi
        ) &
        
        # 显示进度
        echo -n "#"
        
        # 控制并发数
        if [[ $(jobs -r -p | wc -l) -ge 5 ]]
        then
            wait -n
        fi
    done
    
    wait
    echo " 完成"
    
    # 统计结果
    while IFS= read -r line
    do
        if [[ "$line" == SUCCESS:* ]]
        then
            ((SUCCESS_COUNT++))
            log_success "$line"
        elif [[ "$line" == FAIL:* ]]
        then
            ((FAIL_COUNT++))
            log_error "$line"
        fi
    done < "$RESULT_FILE"
    
    # 显示错误详情（如果有）
    if [[ -s "$ERROR_FILE" ]]
    then
        log_warning "并发测试中的错误详情:"
        cat "$ERROR_FILE" | while read error; do
            log_warning "  $error"
        done
    fi
    
    rm -f "$RESULT_FILE" "$ERROR_FILE"
    
    local total=$((SUCCESS_COUNT + FAIL_COUNT))
    local success_rate=0
    if [[ $total -gt 0 ]]
    then
        success_rate=$(echo "scale=2; $SUCCESS_COUNT * 100 / $total" | bc)
    fi
    
    log_success "并发测试完成: 成功 $SUCCESS_COUNT, 失败 $FAIL_COUNT, 成功率: ${success_rate}%"
    
    # 记录测试结果
    if [[ $SUCCESS_COUNT -gt 0 ]]
    then
        log_result "并发测试($concurrent_users用户)" "success" "成功率 ${success_rate}%"
    else
        log_result "并发测试($concurrent_users用户)" "failed" "所有用户都失败"
    fi
    
    echo
}

# 管理功能测试
function admin_functions_test()
{
    log_step "=== 测试管理功能 ==="
    
    local admin_ok=true
    
    # 获取黑名单
    log_info "获取黑名单..."
    RESPONSE=$(curl -s -X GET "$BASE_URL/admin/blacklist?$ADMIN_PARAM")
    log_debug "获取黑名单响应: $RESPONSE"
    if echo "$RESPONSE" | grep -q '"code":0'
    then
        log_success "获取黑名单成功"
        log_result "获取黑名单" "success" "成功获取黑名单"
    else
        log_error "获取黑名单失败: $RESPONSE"
        log_result "获取黑名单" "failed" "获取黑名单失败"
        admin_ok=false
    fi
    
    # 添加用户到黑名单
    log_info "添加用户到黑名单..."
    RESPONSE=$(curl -s -X POST "$BASE_URL/admin/blacklist/add?$ADMIN_PARAM&user_id=9999&reason=test")
    log_debug "添加黑名单响应: $RESPONSE"
    if echo "$RESPONSE" | grep -q '"code":0'
    then
        log_success "添加黑名单成功"
        log_result "添加黑名单" "success" "成功添加用户到黑名单"
    else
        log_error "添加黑名单失败: $RESPONSE"
        log_result "添加黑名单" "failed" "添加黑名单失败"
        admin_ok=false
    fi
    
    # 设置限流
    log_info "设置限流..."
    RESPONSE=$(curl -s -X POST "$BASE_URL/admin/config/rate_limit?$ADMIN_PARAM&limit=50")
    log_debug "设置限流响应: $RESPONSE"
    if echo "$RESPONSE" | grep -q '"code":0'
    then
        log_success "设置限流成功"
        log_result "设置限流" "success" "成功设置限流"
    else
        log_error "设置限流失败: $RESPONSE"
        log_result "设置限流" "failed" "设置限流失败"
        admin_ok=false
    fi
    
    # 设置秒杀开关
    log_info "设置秒杀开关..."
    RESPONSE=$(curl -s -X POST "$BASE_URL/admin/config/seckill/enable?$ADMIN_PARAM&enabled=true")
    log_debug "设置秒杀开关响应: $RESPONSE"
    if echo "$RESPONSE" | grep -q '"code":0'
    then
        log_success "设置秒杀开关成功"
        log_result "设置秒杀开关" "success" "成功开启秒杀"
    else
        log_error "设置秒杀开关失败: $RESPONSE"
        log_result "设置秒杀开关" "failed" "设置秒杀开关失败"
        admin_ok=false
    fi
    
    if $admin_ok; then
        log_success "管理功能测试完成"
    else
        log_warning "管理功能测试部分失败"
    fi
}

# 性能测试
function performance_test()
{
    local concurrent_users=$1
    local goods_id=$2
    
    log_step "=== 开始性能测试 (并发数: $concurrent_users) ==="
    
    START_TIME=$(date +%s)
    
    # 执行并发测试
    concurrent_test $concurrent_users $goods_id
    
    END_TIME=$(date +%s)
    DURATION=$((END_TIME - START_TIME))
    
    log_success "性能测试完成，耗时: ${DURATION}秒"
}

# 主测试流程
function main_test()
{
    log_step "=== 秒杀系统完整测试开始 ==="
    
    # 重置统计
    TOTAL_TESTS=0
    PASSED_TESTS=0
    FAILED_TESTS=0
    
    # 检查服务
    if ! check_service
    then
        exit 1
    fi
    
    # 重置环境
    reset_environment

    # 显示商品信息
    log_info "获取商品信息..."
    GOODS_INFO=$(get_goods_info $GOODS_ID)
    log_debug "商品信息: $GOODS_INFO"
    
    # 测试管理功能
    admin_functions_test
    
    # 单用户测试
    single_user_test 1001 $GOODS_ID
    
    # 并发测试 - 小规模
    performance_test 5 $GOODS_ID
    
    # 并发测试 - 中等规模
    performance_test 10 $GOODS_ID
    
    # 并发测试 - 大规模
    performance_test 20 $GOODS_ID
    
    # 打印最终统计
    print_test_stats
    
    log_success "=== 所有测试完成 ==="
    log_info "详细日志已保存到: $LOG_FILE"
}

# 使用说明
function usage()
{
    echo "用法: $0 [选项]"
    echo "选项:"
    echo "  single    单用户测试"
    echo "  concurrent [数量]  并发测试"
    echo "  admin     管理功能测试"
    echo "  reset     重置环境"
    echo "  all       完整测试(默认)"
    echo ""
    echo "示例:"
    echo "  $0 single              # 单用户测试"
    echo "  $0 concurrent 20       # 20并发测试"
    echo "  $0 admin               # 管理功能测试"
    echo "  $0 reset               # 重置环境"
    echo "  $0 all                 # 完整测试"
}

# 命令行参数处理
case "$1" in
    "single")
        check_service
        single_user_test 1001 $GOODS_ID
        print_test_stats
        ;;
    "concurrent")
        CONCURRENT=${2:-10}
        check_service
        concurrent_test $CONCURRENT $GOODS_ID
        print_test_stats
        ;;
    "admin")
        check_service
        admin_functions_test
        print_test_stats
        ;;
    "reset")
        check_service
        reset_environment
        ;;
    "all"|"")
        main_test
        ;;
    *)
        usage
        ;;
esac
