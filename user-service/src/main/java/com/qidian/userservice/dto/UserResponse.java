package com.qidian.userservice.dto;

/**
 * 用户信息响应体 DTO。
 * <p>
 * 【为什么单独定义一个 UserResponse 而不是直接返回 User 实体】
 * <ul>
 *   <li>安全：{@code User} 实体里有 {@code passwordHash} 字段，绝对不能返回给前端。</li>
 *   <li>解耦：数据库表结构（entity）和对外契约（response）分开演进——
 *       表加字段时不会自动把新字段暴露到 API。</li>
 * </ul>
 */
public record UserResponse(
        Long userId,
        String account,
        String username
) {
}
