package com.qidian.userservice.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;

import java.time.LocalDateTime;

/**
 * 数据库表 {@code users} 对应的实体类。
 * <p>
 * 这个类和数据库表一一映射，字段名和列名通过 MyBatis-Plus 的注解建立关系。
 *
 * <h3>MyBatis-Plus 注解说明</h3>
 * <ul>
 *   <li>{@code @TableName("users")}：绑定到数据库里的表名。
 *       没这个注解的话 MP 会默认按驼峰转下划线，一般也能对上，加上更明确。</li>
 *   <li>{@code @TableId(type = IdType.AUTO)}：标记主键字段，AUTO 表示"数据库自增"，
 *       插入时不需要我们给 id 赋值，插入后 MP 会把生成的 id 自动回写到对象里。</li>
 *   <li>{@code @TableField("password_hash")}：字段名到列名的映射。
 *       Java 里我们用驼峰 {@code passwordHash}，数据库里是下划线 {@code password_hash}，
 *       所以要显式声明。（{@code account} 和 {@code username} 因为大小写恰好都能对上就不用写。）</li>
 * </ul>
 *
 * <h3>为什么保留传统的 getter/setter</h3>
 * 不用 record 是因为 MyBatis-Plus 在很多操作里需要"先无参 new 一个对象再 setXxx 赋值"，
 * 而 record 不能有可变字段。所以 entity 层选择传统 JavaBean 风格。
 */
@TableName("users")
public class User {

    /** 用户主键 ID，数据库自增。 */
    @TableId(type = IdType.AUTO)
    private Long id;

    /** 登录账号（要求唯一），4-32 位。 */
    private String account;

    /**
     * 密码经过 BCrypt 加密后的哈希值。
     * <b>永远不要把这个字段回传到前端</b>。
     */
    @TableField("password_hash")
    private String passwordHash;

    /** 显示昵称。 */
    private String username;

    /** 创建时间，由服务端在 register 时写入。 */
    @TableField("created_at")
    private LocalDateTime createdAt;

    /** 最近更新时间。 */
    @TableField("updated_at")
    private LocalDateTime updatedAt;

    // 下面是标准的 getter / setter。
    // 【为什么必须要有】MyBatis-Plus 用反射读写字段，而它默认走的是 getter/setter 而非直接读字段。
    // IDE 一键生成即可，读起来没什么值得注释的。

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public String getAccount() {
        return account;
    }

    public void setAccount(String account) {
        this.account = account;
    }

    public String getPasswordHash() {
        return passwordHash;
    }

    public void setPasswordHash(String passwordHash) {
        this.passwordHash = passwordHash;
    }

    public String getUsername() {
        return username;
    }

    public void setUsername(String username) {
        this.username = username;
    }

    public LocalDateTime getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(LocalDateTime createdAt) {
        this.createdAt = createdAt;
    }

    public LocalDateTime getUpdatedAt() {
        return updatedAt;
    }

    public void setUpdatedAt(LocalDateTime updatedAt) {
        this.updatedAt = updatedAt;
    }
}
