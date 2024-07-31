global numOfStates; 
% cs = 0,1,2,2'
% la = 0,1
% lh = 0,1
% leader A/H = 0/1
numOfStates = 32;
global alphaPower;

% actions: 1 adopt, 2 wait, 3 release, 4 silent
choices = 4;
adopt = 1; wait = 2; release = 3; silent = 4;

global k;
delta=1;
Delta = delta*k;

global rou latency;
global P T Rc;

%%% transition
P = cell(1,choices);
T = cell(1,choices);
Rc = cell(1,choices);
latency = cell(1,choices);
for i = 1:choices
    P{i} = sparse(numOfStates, numOfStates);
    T{i} = sparse(numOfStates, numOfStates);
    Rc{i} = sparse(numOfStates, numOfStates);
    latency{i} = sparse(numOfStates, numOfStates);
end

H_H_time = 2*delta+Delta;
H_A_time = delta+2*Delta;
A_H_time = 3*Delta;
A_A_time = 3*Delta;
silent_A_time = 2*Delta;

for state = 1:numOfStates
    [cs,la,lh,leader] = tchs_stnum2st(state);
    % next_cs denote result of cs+1
    if cs < 2
        next_cs = cs+1;
    elseif cs==2
        next_cs = 2;
    else % cs==3
        next_cs = 1;
    end
    
    % define adopt
    if leader == 0 % adopt-A
        if la == 0
            P{adopt}(state, tchs_st2stnum(cs,1,0,0)) = alphaPower;
            P{adopt}(state, tchs_st2stnum(cs,1,0,1)) = 1-alphaPower;
            T{adopt}(state, tchs_st2stnum(cs,1,0,0)) = A_A_time;
            T{adopt}(state, tchs_st2stnum(cs,1,0,1)) = A_H_time;
        else
            if cs == 2 || cs ==3
                cs_adopt_a1 = 3;
            else
                cs_adopt_a1 = 0;
            end
            P{adopt}(state, tchs_st2stnum(cs_adopt_a1,1,0,0)) = alphaPower;
            P{adopt}(state, tchs_st2stnum(cs_adopt_a1,1,0,1)) = 1-alphaPower;
            T{adopt}(state, tchs_st2stnum(cs_adopt_a1,1,0,0)) = A_A_time;
            T{adopt}(state, tchs_st2stnum(cs_adopt_a1,1,0,1)) = A_H_time;
        end
    else % adopt-H
        if la == 0
            cs_adopt_h = next_cs;
        else
            cs_adopt_h = 1;
        end
        P{adopt}(state, tchs_st2stnum(cs_adopt_h,0,1,0)) = alphaPower;
        P{adopt}(state, tchs_st2stnum(cs_adopt_h,0,1,1)) = 1-alphaPower;
        T{adopt}(state, tchs_st2stnum(cs_adopt_h,0,1,0)) = H_A_time;
        T{adopt}(state, tchs_st2stnum(cs_adopt_h,0,1,1)) = H_H_time;
        if cs == 2 || cs == 3
            Rc{adopt}(state, tchs_st2stnum(cs_adopt_h,0,1,0)) = 1;
            Rc{adopt}(state, tchs_st2stnum(cs_adopt_h,0,1,1)) = 1;
        end
    end
    
    
    % define wait
    if leader == 0 % wait-A
        if la == 0
            if cs == 2 || cs ==3
                cs_wait_a0 = 3;
            else
                cs_wait_a0 = 0;
            end
            P{wait}(state, tchs_st2stnum(cs_wait_a0,1,lh,0)) = alphaPower;
            P{wait}(state, tchs_st2stnum(cs_wait_a0,1,lh,1)) = 1-alphaPower;
            T{wait}(state, tchs_st2stnum(cs_wait_a0,1,lh,0)) = A_A_time;
            T{wait}(state, tchs_st2stnum(cs_wait_a0,1,lh,1)) = A_H_time;
        else
            if lh > 0
                P{wait}(state, tchs_st2stnum(1,1,0,0)) = alphaPower;
                P{wait}(state, tchs_st2stnum(1,1,0,1)) = 1-alphaPower;
                T{wait}(state, tchs_st2stnum(1,1,0,0)) = A_A_time;
                T{wait}(state, tchs_st2stnum(1,1,0,1)) = A_H_time;
            else
                P{wait}(state, tchs_st2stnum(next_cs,1,0,0)) = alphaPower;
                P{wait}(state, tchs_st2stnum(next_cs,1,0,1)) = 1-alphaPower;
                T{wait}(state, tchs_st2stnum(next_cs,1,0,0)) = A_A_time;
                T{wait}(state, tchs_st2stnum(next_cs,1,0,1)) = A_H_time;
                if cs == 2 || cs == 3
                    Rc{wait}(state, tchs_st2stnum(next_cs,1,0,0)) = 1;
                    Rc{wait}(state, tchs_st2stnum(next_cs,1,0,1)) = 1;
                end
            end
        end
    else % wait - H
        if la == 0
            cs_wait_h = next_cs;
        else % la == 1
            cs_wait_h = 1;
        end
        if lh == 1
            P{wait}(state, tchs_st2stnum(cs_wait_h,0,lh,0)) = alphaPower;
            P{wait}(state, tchs_st2stnum(cs_wait_h,0,lh,1)) = 1-alphaPower;
            T{wait}(state, tchs_st2stnum(cs_wait_h,0,lh,0)) = H_A_time;
            T{wait}(state, tchs_st2stnum(cs_wait_h,0,lh,1)) = H_H_time;
            if cs == 2 || cs == 3
                Rc{wait}(state, tchs_st2stnum(cs_wait_h,0,lh,0)) = 1;
                Rc{wait}(state, tchs_st2stnum(cs_wait_h,0,lh,1)) = 1;
            end
        else
            P{wait}(state, tchs_st2stnum(cs_wait_h,0,lh+1,0)) = alphaPower;
            P{wait}(state, tchs_st2stnum(cs_wait_h,0,lh+1,1)) = 1-alphaPower;
            T{wait}(state, tchs_st2stnum(cs_wait_h,0,lh+1,0)) = H_A_time;
            T{wait}(state, tchs_st2stnum(cs_wait_h,0,lh+1,1)) = H_H_time;
            if cs == 2 || cs == 3
                Rc{wait}(state, tchs_st2stnum(cs_wait_h,0,lh+1,0)) = 1;
                Rc{wait}(state, tchs_st2stnum(cs_wait_h,0,lh+1,1)) = 1;
            end
        end
    end
    
    % define release
    if la == 1
        if leader == 0 % release-A
            if lh > 0
                cs_release_a = 1;
            else
                cs_release_a = next_cs;
            end
            P{release}(state, tchs_st2stnum(cs_release_a,1,0,0)) = alphaPower;
            P{release}(state, tchs_st2stnum(cs_release_a,1,0,1)) = 1-alphaPower;
            T{release}(state, tchs_st2stnum(cs_release_a,1,0,0)) = A_A_time;
            T{release}(state, tchs_st2stnum(cs_release_a,1,0,1)) = A_H_time;
            if lh == 0
                if cs == 2 || cs == 3
                    Rc{release}(state, tchs_st2stnum(cs_release_a,1,0,0)) = 1;
                    Rc{release}(state, tchs_st2stnum(cs_release_a,1,0,1)) = 1;
                end
            end
        else % release-H
            P{release}(state, tchs_st2stnum(2,0,1,0)) = alphaPower;
            P{release}(state, tchs_st2stnum(2,0,1,1)) = 1-alphaPower;
            T{release}(state, tchs_st2stnum(2,0,1,0)) = H_A_time;
            T{release}(state, tchs_st2stnum(2,0,1,1)) = H_H_time;
            if lh == 0
                if cs == 2
                    Rc{release}(state, tchs_st2stnum(2,0,1,0)) = 2;
                    Rc{release}(state, tchs_st2stnum(2,0,1,1)) = 2;
                elseif cs == 1 || cs == 3
                    Rc{release}(state, tchs_st2stnum(2,0,1,0)) = 1;
                    Rc{release}(state, tchs_st2stnum(2,0,1,1)) = 1;
                end
            end
        end
    else
        % for completeness
        P{release}(state, 1) = 1;
        Rc{release}(state, 1) = 10000;
        T{release}(state, 1) = A_A_time;
    end
    
    % define silent
    if leader == 0 % silent - A
        if la == 0
            if lh > 0 && cs ~= 0 && cs ~= 2
                P{silent}(state, tchs_st2stnum(0,0,lh-1,0)) = alphaPower;
                P{silent}(state, tchs_st2stnum(0,0,lh-1,1)) = 1-alphaPower;
                T{silent}(state, tchs_st2stnum(0,0,lh-1,0)) = silent_A_time;
                T{silent}(state, tchs_st2stnum(0,0,lh-1,1)) = silent_A_time;
            else 
                P{silent}(state, tchs_st2stnum(0,0,lh,0)) = alphaPower;
                P{silent}(state, tchs_st2stnum(0,0,lh,1)) = 1-alphaPower;
                T{silent}(state, tchs_st2stnum(0,0,lh,0)) = silent_A_time;
                T{silent}(state, tchs_st2stnum(0,0,lh,1)) = silent_A_time;
            end
        else % la == 1
            P{silent}(state, tchs_st2stnum(0,0,lh,0)) = alphaPower;
            P{silent}(state, tchs_st2stnum(0,0,lh,1)) = 1-alphaPower;
            T{silent}(state, tchs_st2stnum(0,0,lh,0)) = silent_A_time;
            T{silent}(state, tchs_st2stnum(0,0,lh,1)) = silent_A_time;
        end
    else % silent - H
        if la == 0
            cs_silent_h = next_cs;
        else % la == 1
            cs_silent_h = 1;
        end
        if lh == 1
            P{silent}(state, tchs_st2stnum(cs_silent_h,0,lh,0)) = alphaPower;
            P{silent}(state, tchs_st2stnum(cs_silent_h,0,lh,1)) = 1-alphaPower;
            T{silent}(state, tchs_st2stnum(cs_silent_h,0,lh,0)) = H_A_time;
            T{silent}(state, tchs_st2stnum(cs_silent_h,0,lh,1)) = H_H_time;
            if cs == 2 || cs == 3
                Rc{silent}(state, tchs_st2stnum(cs_silent_h,0,lh,0)) = 1;
                Rc{silent}(state, tchs_st2stnum(cs_silent_h,0,lh,1)) = 1;
            end
        else
            P{silent}(state, tchs_st2stnum(cs_silent_h,0,lh+1,0)) = alphaPower;
            P{silent}(state, tchs_st2stnum(cs_silent_h,0,lh+1,1)) = 1-alphaPower;
            T{silent}(state, tchs_st2stnum(cs_silent_h,0,lh+1,0)) = H_A_time;
            T{silent}(state, tchs_st2stnum(cs_silent_h,0,lh+1,1)) = H_H_time;
            if cs == 2 || cs == 3
                Rc{silent}(state, tchs_st2stnum(cs_silent_h,0,lh+1,0)) = 1;
                Rc{silent}(state, tchs_st2stnum(cs_silent_h,0,lh+1,1)) = 1;
            end
        end
    end
    
end

disp(mdp_check(P, Rc))

epsilon = 0.0001;

lowRou = 0;
highRou = 1;
while(highRou - lowRou > epsilon/8)
    rou = (highRou + lowRou) / 2;
    for i = 1:choices
        latency{i} = (1-rou).*T{i} - Rc{i};
    end
    [latencyPolicy, reward, cpuTime] = mdp_relative_value_iteration(P, latency, epsilon/8);
    if(reward > 0)
        lowRou = rou;
    else
        highRou = rou;
    end
end
disp('Latency: ')
format long
disp(1-rou)
