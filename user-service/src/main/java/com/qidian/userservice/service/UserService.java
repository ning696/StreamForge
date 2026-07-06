package com.qidian.userservice.service;

import com.qidian.userservice.dto.LoginRequest;
import com.qidian.userservice.dto.RegisterRequest;
import com.qidian.userservice.dto.UserResponse;

public interface UserService {

    UserResponse register(RegisterRequest request);

    UserResponse login(LoginRequest request);

    UserResponse getUser(Long userId);
}
