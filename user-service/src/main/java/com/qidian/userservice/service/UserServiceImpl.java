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

@Service
public class UserServiceImpl implements UserService {

    private final UserMapper userMapper;
    private final PasswordEncoder passwordEncoder;

    public UserServiceImpl(UserMapper userMapper, PasswordEncoder passwordEncoder) {
        this.userMapper = userMapper;
        this.passwordEncoder = passwordEncoder;
    }

    @Override
    public UserResponse register(RegisterRequest request) {
        String account = normalize(request.account());
        String username = normalize(request.username());
        validateAccount(account);
        validatePassword(request.password());
        validateUsername(username);

        Long count = userMapper.selectCount(new LambdaQueryWrapper<User>().eq(User::getAccount, account));
        if (count != null && count > 0) {
            throw new BusinessException("ACCOUNT_EXISTS", "账号已存在", HttpStatus.CONFLICT);
        }

        LocalDateTime now = LocalDateTime.now();
        User user = new User();
        user.setAccount(account);
        user.setPasswordHash(passwordEncoder.encode(request.password()));
        user.setUsername(username);
        user.setCreatedAt(now);
        user.setUpdatedAt(now);

        try {
            userMapper.insert(user);
        } catch (DuplicateKeyException exception) {
            throw new BusinessException("ACCOUNT_EXISTS", "账号已存在", HttpStatus.CONFLICT);
        }
        return toResponse(user);
    }

    @Override
    public UserResponse login(LoginRequest request) {
        String account = normalize(request.account());
        validateAccount(account);
        validatePassword(request.password());

        User user = userMapper.selectOne(new LambdaQueryWrapper<User>().eq(User::getAccount, account));
        if (user == null || !passwordEncoder.matches(request.password(), user.getPasswordHash())) {
            throw new BusinessException("INVALID_CREDENTIALS", "账号或密码错误", HttpStatus.UNAUTHORIZED);
        }
        return toResponse(user);
    }

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

    private UserResponse toResponse(User user) {
        return new UserResponse(user.getId(), user.getAccount(), user.getUsername());
    }

    private String normalize(String value) {
        return value == null ? "" : value.trim();
    }

    private void validateAccount(String account) {
        if (!StringUtils.hasText(account) || account.length() < 4 || account.length() > 32) {
            throw new BusinessException("BAD_REQUEST", "账号长度必须为4-32", HttpStatus.BAD_REQUEST);
        }
    }

    private void validatePassword(String password) {
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
