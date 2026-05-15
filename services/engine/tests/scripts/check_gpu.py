import torch
import sys

def check_gpu():
    print(f"Python Version: {sys.version}")
    print(f"PyTorch Version: {torch.__version__}")
    print(f"CUDA Available: {torch.cuda.is_available()}")
    
    if torch.cuda.is_available():
        print(f"CUDA Version (PyTorch): {torch.version.cuda}")
        print(f"Device Name: {torch.cuda.get_device_name(0)}")
        
        # Get compute capability (Blackwell should be 12.0)
        capability = torch.cuda.get_device_capability(0)
        print(f"Compute Capability: {capability[0]}.{capability[1]}")
        
        # Test a simple tensor operation to verify kernel execution
        try:
            x = torch.randn(10, 10).cuda()
            y = torch.randn(10, 10).cuda()
            z = x @ y
            print("✅ CUDA Kernel Execution: SUCCESS")
        except Exception as e:
            print(f"❌ CUDA Kernel Execution: FAILED")
            print(f"Error: {e}")
            
        # Check supported architectures
        if hasattr(torch.cuda, "get_arch_list"):
            print(f"Supported Architectures: {torch.cuda.get_arch_list()}")
            if "sm_120" in torch.cuda.get_arch_list():
                print("✅ Blackwell (sm_120) support found in this build.")
            else:
                print("⚠️ Blackwell (sm_120) NOT found in supported architectures.")
    else:
        print("❌ CUDA is NOT available. Check your drivers and PyTorch installation.")

if __name__ == "__main__":
    check_gpu()
