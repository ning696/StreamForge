package com.qidian.userservice.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.qidian.userservice.dto.LoginRequest;
import com.qidian.userservice.dto.RegisterRequest;
import com.qidian.userservice.dto.UserResponse;
import com.qidian.userservice.entity.User;
import com.qidian.userservice.exception.BusinessException;
import com.qidian.userservice.mapper.UserMapper;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.http.HttpStatus;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;

import java.time.LocalDateTime;

/**
 * 用户业务的具体实现。
 * <p>
 * 【@Service 的作用】
 * 告诉 Spring "这是一个业务层组件"，扫描时把它实例化并放入容器。
 * Controller 注入 {@link UserService} 时，Spring 会找到本类作为实现注入进去。
 *
 * <h3>方法一览</h3>
 * <ul>
 *   <li>{@link #register(RegisterRequest)}：写入新用户</li>
 *   <li>{@link #login(LoginRequest)}：校验账号密码</li>
 *   <li>{@link #getUser(Long)}：按 ID 查用户</li>
 * </ul>
 *
 * <h3>为什么校验既在 DTO 上又在这里写一遍</h3>
 * <ul>
 *   <li>DTO 上的 {@code @NotBlank/@Size} 只在 Controller 入口跑一次。</li>
 *   <li>Service 里再校验一次是为了：如果未来有别的调用方（比如内部服务间调用）绕过 Controller
 *       直接调 Service，也不会写入垃圾数据。这是"深度防御"的思路。</li>
 * </ul>
 */
@Service
public class UserServiceImpl implements UserService {

    private final UserMapper userMapper;
    private final PasswordEncoder passwordEncoder;

    /**
     * 构造函数注入。Spring 会自动把容器里的 UserMapper 和 PasswordEncoder 传进来。
     */
    public UserServiceImpl(UserMapper userMapper, PasswordEncoder passwordEncoder) {
        this.userMapper = userMapper;
        this.passwordEncoder = passwordEncoder;
    }

    /**
     * 注册流程：
     * <ol>
     *   <li>规范化 & 校验参数</li>
     *   <li>先查一遍账号是否已存在（"友好路径"，能返回明确错误）</li>
     *   <li>加密密码后插入数据库</li>
     *   <li>再用 {@link DuplicateKeyException} 兜底并发场景（两个人同时注册同一个账号）</li>
     * </ol>
     */
    @Override
    public UserResponse register(RegisterRequest request) {
        String account = normalize(request.account());
        String username = normalize(request.username());
        validateAccount(account);
        validatePassword(request.password());
        validateUsername(username);

        // 【LambdaQueryWrapper】MyBatis-Plus 提供的"类型安全"的条件构造器。
        // 用方法引用 User::getAccount 代替字符串列名，重构改名时编译器会帮我们发现问题。
        // 这里生成的 SQL 相当于：SELECT COUNT(*) FROM users WHERE account = ?
        Long count = userMapper.selectCount(new LambdaQueryWrapper<User>().eq(User::getAccount, account));
        if (count != null && count > 0) {
            // HttpStatus.CONFLICT (409)：语义正好是"资源冲突/已存在"。
            throw new BusinessException("ACCOUNT_EXISTS", "账号已存在", HttpStatus.CONFLICT);
        }

        // 构造实体。id 不用赋值——数据库自增。
        LocalDateTime now = LocalDateTime.now();
        User user = new User();
        user.setAccount(account);
        // 【重要】永远存加密后的哈希，不要存明文密码。
        user.setPasswordHash(passwordEncoder.encode(request.password()));
        user.setUsername(username);
        user.setCreatedAt(now);
        user.setUpdatedAt(now);

        try {
            // insert 成功后 MyBatis-Plus 会把生成的自增 id 回填到 user.id 字段。
            userMapper.insert(user);
        } catch (DuplicateKeyException exception) {
            // 【并发保护】上面 selectCount + 下面 insert 中间有短暂的"竞态窗口"。
            // 如果两个请求同时进来、都查到 count=0，第二次 insert 就会撞到数据库唯一约束。
            // 这里把底层的 DuplicateKeyException 翻译成语义化的业务异常。
            throw new BusinessException("ACCOUNT_EXISTS", "账号已存在", HttpStatus.CONFLICT);
        }
        return toResponse(user);
    }

    /**
     * 登录流程：
     * <ol>
     *   <li>规范化 + 参数校验</li>
     *   <li>按 account 查用户</li>
     *   <li>用 BCrypt 校验密码</li>
     * </ol>
     * <p>
     * 【安全考虑】"用户不存在"和"密码错误"返回同一个错误码/消息，
     * 避免攻击者通过响应差异探测哪些账号确实存在。
     */
    @Override
    public UserResponse login(LoginRequest request) {
        String account = normalize(request.account());
        validateAccount(account);
        validatePassword(request.password());

        User user = userMapper.selectOne(new LambdaQueryWrapper<User>().eq(User::getAccount, account));
        // passwordEncoder.matches(明文, 哈希) 内部会用同样的算法/盐重新算，再比对。
        if (user == null || !passwordEncoder.matches(request.password(), user.getPasswordHash())) {
            throw new BusinessException("INVALID_CREDENTIALS", "账号或密码错误", HttpStatus.UNAUTHORIZED);
        }
        return toResponse(user);
    }

    /**
     * 按 ID 查用户。
     * <p>
     * 【为什么单独判断 userId <= 0】
     * 走到这里的通常是 Controller 传来的 PathVariable，Spring 已经保证非 null，
     * 但仍要防御 0 或负数（比如内部调用手写传参出错）。
     */
    @Override
    public UserResponse getUser(Long userId) {
        if (userId == null || userId <= 0) {
            throw new BusinessException("USER_NOT_FOUND", "用户不存在", HttpStatus.NOT_FOUND);
        }
        User user = userMapper.selectById(userId);
        if (user == null) {
            throw new BusinessException("USER_NOT_FOUND", "用户不存在", HttpStatus.NOT_FOUND);
        }
        return toResponse(user);
    }

    // ============ 以下是私有辅助方法 ============

    /**
     * 把实体转成对外响应 DTO，屏蔽掉 passwordHash 等敏感字段。
     */
    private UserResponse toResponse(User user) {
        return new UserResponse(user.getId(), user.getAccount(), user.getUsername());
    }

    /**
     * 归一化字符串：null -> ""，其他 -> trim。
     * 让"空格账号""前后有空格"等异常输入不再引起后续 NPE 或误存。
     */
    private String normalize(String value) {
        return value == null ? "" : value.trim();
    }

    private void validateAccount(String account) {
        // StringUtils.hasText：非 null 且去空白后非空。
        if (!StringUtils.hasText(account) || account.length() < 4 || account.length() > 32) {
            throw new BusinessException("BAD_REQUEST", "账号长度必须为4-32", HttpStatus.BAD_REQUEST);
        }
    }

    private void validatePassword(String password) {
        // 注意这里不 trim 密码——空格也是有效的密码字符，不能悄悄改动用户输入。
        if (password == null || password.length() < 6 || password.length() > 64) {
            throw new BusinessException("BAD_REQUEST", "密码长度必须为6-64", HttpStatus.BAD_REQUEST);
        }
    }

    private void validateUsername(String username) {
        if (!StringUtils.hasText(username) || username.length() > 32) {
            throw new BusinessException("BAD_REQUEST", "用户名长度必须为1-32", HttpStatus.BAD_REQUEST);
        }
    }
}
