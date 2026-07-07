package com.qidian.userservice.service;

import com.qidian.userservice.dto.LoginRequest;
import com.qidian.userservice.dto.RegisterRequest;
import com.qidian.userservice.dto.UserResponse;

/**
 * 用户业务服务接口。
 * <p>
 * 【为什么接口 + 实现分离】
 * <ol>
 *   <li>让 Controller 依赖抽象而不是具体实现，未来如果实现方式变了（比如从 MyBatis 换成 JPA），
 *       Controller 一行都不用改。</li>
 *   <li>方便单元测试用 Mock 实现替换真实实现。</li>
 *   <li>Spring 的 AOP（比如声明式事务、缓存）是基于接口代理更省事的，虽然 CGLIB 也能代理类。</li>
 * </ol>
 */
public interface UserService {

    /**
     * 注册新账号。
     *
     * @param request 注册请求（账号、密码、昵称）
     * @return 新用户的公开信息
     * @throws com.qidian.userservice.exception.BusinessException 账号已存在或参数不合规时抛出
     */
    UserResponse register(RegisterRequest request);

    /**
     * 账号密码登录。
     *
     * @return 用户公开信息
     * @throws com.qidian.userservice.exception.BusinessException 账号或密码错误时抛出
     */
    UserResponse login(LoginRequest request);

    /**
     * 按 ID 查用户。
     *
     * @throws com.qidian.userservice.exception.BusinessException 用户不存在时抛出
     */
    UserResponse getUser(Long userId);
}
